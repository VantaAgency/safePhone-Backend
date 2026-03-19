package middleware

import (
	"strings"

	"github.com/go-chi/cors"
)

// CORSHandler creates a chi-compatible CORS handler.
func CORSHandler(originsCSV string) *cors.Cors {
	origins := strings.Split(originsCSV, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	return cors.New(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Idempotency-Key", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID", "Retry-After"},
		AllowCredentials: true,
		MaxAge:           600,
	})
}
