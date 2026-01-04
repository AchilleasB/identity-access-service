package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/config"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
	"github.com/lib/pq"
	"github.com/sony/gobreaker"
)

const (
	// PostgreSQL NOTIFY/LISTEN configuration
	listenerMinReconnectInterval = 10 * time.Second
	listenerMaxReconnectInterval = time.Minute
	outboxChannelName            = "outbox_channel"

	// Event processing timeouts
	eventProcessTimeout     = 30 * time.Second
	batchProcessTimeout     = 60 * time.Second
	periodicProcessInterval = 90 * time.Second

	// Health check configuration
	healthCheckStaleThreshold = 5 * time.Minute

	// Batch processing limits
	maxEventsPerBatch = 100
)

// Relay listens for PostgreSQL NOTIFY signals on the outbox_channel
// and publishes events to RabbitMQ.
type Relay struct {
	db            *sql.DB
	publisher     ports.BabyEventPublisher
	listener      *pq.Listener
	dbURL         string
	dbCB          *gobreaker.CircuitBreaker
	lastProcessed time.Time
	isHealthy     bool
}

// NewRelay creates a new outbox relay that listens for PostgreSQL notifications.
func NewRelay(db *sql.DB, dbURL string, publisher ports.BabyEventPublisher) *Relay {
	// Configure circuit breaker for database operations
	dbCB := config.NewCircuitBreaker("Relay-PostgreSQL")

	return &Relay{
		db:            db,
		dbURL:         dbURL,
		publisher:     publisher,
		dbCB:          dbCB,
		lastProcessed: time.Now(),
		isHealthy:     true,
	}
}

// IsHealthy returns true if the relay process is alive and responding.
// This is designed for Liveness probes - keeps checks simple to avoid false positives.
// For Readiness probes, you should check circuit breaker state and dependency health.
func (r *Relay) IsHealthy() bool {
	// Simple check: is the process responsive?
	// We don't check circuit breaker state here because:
	// - Open circuit = degraded but recoverable (shouldn't kill pod)
	// - Liveness is about "is process alive", not "is system healthy"
	return r.isHealthy
}

// IsReady returns true if the relay can process events (for readiness probes).
func (r *Relay) IsReady() bool {
	// Check if circuit breaker is open (system is degraded)
	if r.dbCB.State() == gobreaker.StateOpen {
		return false
	}

	// Check if we've processed something recently (not stuck)
	if time.Since(r.lastProcessed) > healthCheckStaleThreshold {
		return false
	}

	return r.isHealthy
}

// Start begins listening for outbox notifications and processing events.
// This is a blocking call that runs until the context is cancelled.
func (r *Relay) Start(ctx context.Context) error {
	// Create a listener for PostgreSQL NOTIFY events
	reportProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			log.Printf("outbox relay: listener error: %v", err)
		}
	}

	r.listener = pq.NewListener(r.dbURL, listenerMinReconnectInterval, listenerMaxReconnectInterval, reportProblem)
	defer r.listener.Close()

	if err := r.listener.Listen(outboxChannelName); err != nil {
		return err
	}

	log.Printf("outbox relay: listening on '%s' for notifications...", outboxChannelName)

	// Process any unprocessed events on startup (catch-up)
	if err := r.processUnprocessedEvents(ctx); err != nil {
		log.Printf("outbox relay: error processing startup backlog: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("outbox relay: shutting down...")
			return ctx.Err()

		case notification := <-r.listener.Notify:
			if notification == nil {
				log.Println("outbox relay: received nil notification (reconnecting...)")
				r.isHealthy = false
				continue
			}

			log.Printf("outbox relay: received notification for event ID: %s", notification.Extra)

			// Process the specific event by ID
			if err := r.processEventByID(ctx, notification.Extra); err != nil {
				log.Printf("outbox relay: error processing event %s: %v", notification.Extra, err)
			} else {
				r.lastProcessed = time.Now()
				r.isHealthy = true
			}

		case <-time.After(periodicProcessInterval):
			// Periodic ping to keep connection alive and catch any missed events
			go r.listener.Ping()

			// Also process any unprocessed events (safety net)
			if err := r.processUnprocessedEvents(ctx); err != nil {
				log.Printf("outbox relay: error in periodic processing: %v", err)
			} else {
				r.lastProcessed = time.Now()
			}
		}
	}
}

// processEventByID processes a single event by its ID.
func (r *Relay) processEventByID(ctx context.Context, eventID string) error {
	ctx, cancel := context.WithTimeout(ctx, eventProcessTimeout)
	defer cancel()

	// Wrap database transaction in circuit breaker
	_, err := r.dbCB.Execute(func() (interface{}, error) {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()

		// Lock and fetch the event
		var id, eventType string
		var payload []byte
		err = tx.QueryRowContext(ctx, `
			SELECT id, event_type, payload
			FROM outbox_events
			WHERE id = $1 AND processed_at IS NULL
			FOR UPDATE SKIP LOCKED`, eventID).Scan(&id, &eventType, &payload)

		if err == sql.ErrNoRows {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}

		eventQueue := os.Getenv("BABY_QUEUE_NAME")
		if eventType == eventQueue {
			var evt ports.CreateBabyEvent
			if err := json.Unmarshal(payload, &evt); err != nil {
				log.Printf("outbox relay: invalid payload for event %s: %v", id, err)
				// Mark as processed anyway to avoid infinite retries on bad data
				_, _ = tx.ExecContext(ctx, `UPDATE outbox_events SET processed_at = NOW() WHERE id = $1`, id)
				return nil, tx.Commit()
			}

			if err := r.publisher.PublishBabyCreated(ctx, evt); err != nil {
				return nil, err
			}
		}

		if _, err := tx.ExecContext(ctx, `UPDATE outbox_events SET processed_at = NOW() WHERE id = $1`, id); err != nil {
			return nil, err
		}

		return nil, tx.Commit()
	})
	return err
}

// processUnprocessedEvents processes all unprocessed events (catch-up/recovery).
func (r *Relay) processUnprocessedEvents(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, batchProcessTimeout)
	defer cancel()

	// Wrap database transaction in circuit breaker
	_, err := r.dbCB.Execute(func() (interface{}, error) {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()

		rows, err := tx.QueryContext(ctx, `
			SELECT id, event_type, payload
			FROM outbox_events
			WHERE processed_at IS NULL
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED`, maxEventsPerBatch)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		type record struct {
			ID        string
			EventType string
			Payload   []byte
		}

		var records []record
		for rows.Next() {
			var r record
			if err := rows.Scan(&r.ID, &r.EventType, &r.Payload); err != nil {
				return nil, err
			}
			records = append(records, r)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

		for _, rec := range records {
			eventQueue := os.Getenv("BABY_QUEUE_NAME")
			if rec.EventType == eventQueue {
				var evt ports.CreateBabyEvent
				if err := json.Unmarshal(rec.Payload, &evt); err != nil {
					log.Printf("outbox relay: invalid payload for event %s: %v", rec.ID, err)
					_, _ = tx.ExecContext(ctx, `UPDATE outbox_events SET processed_at = NOW() WHERE id = $1`, rec.ID)
					continue
				}

				if err := r.publisher.PublishBabyCreated(ctx, evt); err != nil {
					log.Printf("outbox relay: failed to publish event %s: %v", rec.ID, err)
					continue
				}
			}

			if _, err := tx.ExecContext(ctx, `UPDATE outbox_events SET processed_at = NOW() WHERE id = $1`, rec.ID); err != nil {
				return nil, err
			}

			log.Printf("outbox relay: processed event %s", rec.ID)
		}

		return nil, tx.Commit()
	})
	return err
}
