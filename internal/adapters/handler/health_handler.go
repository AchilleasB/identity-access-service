package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	db          *sql.DB
	redisClient *redis.Client
	startTime   time.Time
	version     string
}

func NewHealthHandler(db *sql.DB, redisClient *redis.Client) *HealthHandler {
	version := os.Getenv("APP_VERSION")
	if version == "" {
		version = "unknown"
	}
	return &HealthHandler{
		db:          db,
		redisClient: redisClient,
		startTime:   time.Now(),
		version:     version,
	}
}

// HealthResponse follows Kubernetes/OpenShift health check conventions
type HealthResponse struct {
	Status    string           `json:"status"`
	Timestamp string           `json:"timestamp"`
	Uptime    string           `json:"uptime"`
	Version   string           `json:"version"`
	Checks    map[string]Check `json:"checks"`
}

type Check struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// Health is a simple liveness check - just confirms the Go process is running
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := HealthResponse{
		Status:    "UP",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Uptime:    time.Since(h.startTime).Round(time.Second).String(),
		Version:   h.version,
		Checks:    map[string]Check{"process": {Status: "UP"}},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// Ready checks if the service is ready to accept traffic (readiness probe)
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	checks := make(map[string]Check)
	status := "UP"
	httpStatus := http.StatusOK

	dbCheck := h.checkDatabase()
	checks["database"] = dbCheck
	if dbCheck.Status != "UP" {
		status = "DOWN"
		httpStatus = http.StatusServiceUnavailable
	}

	redisCheck := h.checkRedis()
	checks["redis"] = redisCheck
	if redisCheck.Status != "UP" {
		status = "DOWN"
		httpStatus = http.StatusServiceUnavailable
	}

	response := map[string]interface{}{
		"status": status,
		"checks": checks,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// Live is an alias for Health - simple liveness check
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	h.Health(w, r)
}

func (h *HealthHandler) checkDatabase() Check {
	if h.db == nil {
		return Check{
			Status:  "DOWN",
			Message: "Database connection is not initialized",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		return Check{
			Status:  "DOWN",
			Message: "Cannot connect to database",
		}
	}
	return Check{Status: "UP"}
}

func (h *HealthHandler) checkRedis() Check {
	if h.redisClient == nil {
		return Check{
			Status:  "DOWN",
			Message: "Redis client is not initialized",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.redisClient.Ping(ctx).Err(); err != nil {
		return Check{
			Status:  "DOWN",
			Message: "Cannot connect to Redis",
		}
	}
	return Check{Status: "UP"}
}
