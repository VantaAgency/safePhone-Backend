package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/cache"
)

// HealthHandler provides health check endpoints.
type HealthHandler struct {
	pool  *pgxpool.Pool
	redis *cache.Client
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(pool *pgxpool.Pool, redis *cache.Client) *HealthHandler {
	return &HealthHandler{pool: pool, redis: redis}
}

// Live returns 200 if the server is running.
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready returns 200 only if both DB and Redis are reachable.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if err := h.pool.Ping(r.Context()); err != nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unhealthy",
			"reason": "database unreachable",
		})
		return
	}

	if _, err := h.redis.Underlying().Ping(r.Context()).Result(); err != nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unhealthy",
			"reason": "redis unreachable",
		})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
