package auth

import (
	"net/http"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/respond"
)

// RequireRole returns middleware that rejects requests from users without an allowed role.
func RequireRole(allowed ...Role) func(http.Handler) http.Handler {
	allowedSet := make(map[Role]bool, len(allowed))
	for _, r := range allowed {
		allowedSet[r] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, err := GetAuthContext(r.Context())
			if err != nil {
				respond.Error(w, r, domain.Unauthorized("authentication required"))
				return
			}

			allowed := allowedSet[ac.Role]
			if !allowed {
				for _, role := range ac.Roles {
					if allowedSet[role] {
						allowed = true
						break
					}
				}
			}
			if !allowed {
				respond.Error(w, r, domain.Forbidden("insufficient permissions"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
