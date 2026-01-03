package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
	"github.com/lib/pq"
)

// Relay listens for PostgreSQL NOTIFY signals on the outbox_channel
// and publishes events to RabbitMQ.
type Relay struct {
	db        *sql.DB
	publisher ports.BabyEventPublisher
	listener  *pq.Listener
	dbURL     string
}

// NewRelay creates a new outbox relay that listens for PostgreSQL notifications.
func NewRelay(db *sql.DB, dbURL string, publisher ports.BabyEventPublisher) *Relay {
	return &Relay{
		db:        db,
		dbURL:     dbURL,
		publisher: publisher,
	}
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

	r.listener = pq.NewListener(r.dbURL, 10*time.Second, time.Minute, reportProblem)
	defer r.listener.Close()

	if err := r.listener.Listen("outbox_channel"); err != nil {
		return err
	}

	log.Println("outbox relay: listening on 'outbox_channel' for notifications...")

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
				// Connection lost, listener will reconnect automatically
				log.Println("outbox relay: received nil notification (reconnecting...)")
				continue
			}

			log.Printf("outbox relay: received notification for event ID: %s", notification.Extra)

			// Process the specific event by ID
			if err := r.processEventByID(ctx, notification.Extra); err != nil {
				log.Printf("outbox relay: error processing event %s: %v", notification.Extra, err)
			}

		case <-time.After(90 * time.Second):
			// Periodic ping to keep connection alive and catch any missed events
			go r.listener.Ping()

			// Also process any unprocessed events (safety net)
			if err := r.processUnprocessedEvents(ctx); err != nil {
				log.Printf("outbox relay: error in periodic processing: %v", err)
			}
		}
	}
}

// processEventByID processes a single event by its ID.
func (r *Relay) processEventByID(ctx context.Context, eventID string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
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
		// Already processed or doesn't exist
		return nil
	}
	if err != nil {
		return err
	}

	// Publish based on event type
	eventQueue := os.Getenv("BABY_QUEUE_NAME")
	if eventType == eventQueue {
		var evt ports.CreateBabyEvent
		if err := json.Unmarshal(payload, &evt); err != nil {
			log.Printf("outbox relay: invalid payload for event %s: %v", id, err)
			// Mark as processed anyway to avoid infinite retries on bad data
			_, _ = tx.ExecContext(ctx, `UPDATE outbox_events SET processed_at = NOW() WHERE id = $1`, id)
			return tx.Commit()
		}

		if err := r.publisher.PublishBabyCreated(ctx, evt); err != nil {
			return err
		}
	}

	// Mark as processed
	if _, err := tx.ExecContext(ctx, `UPDATE outbox_events SET processed_at = NOW() WHERE id = $1`, id); err != nil {
		return err
	}

	return tx.Commit()
}

// processUnprocessedEvents processes all unprocessed events (catch-up/recovery).
func (r *Relay) processUnprocessedEvents(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, event_type, payload
		FROM outbox_events
		WHERE processed_at IS NULL
		ORDER BY created_at
		LIMIT 100
		FOR UPDATE SKIP LOCKED`)
	if err != nil {
		return err
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
			return err
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return err
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
			return err
		}

		log.Printf("outbox relay: processed event %s", rec.ID)
	}

	return tx.Commit()
}
