// Package auth provides JWT verification, AuthContext, and RBAC middleware.
package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// Role represents a user's role within the system.
type Role string

const (
	RoleAdmin   Role = "admin"
	RoleMember  Role = "member"
	RolePartner Role = "partner"
	RoleViewer  Role = "viewer"
)

// AuthContext holds the authenticated user's identity extracted from a JWT.
// Services read this from context — never from handler parameters.
type AuthContext struct {
	UserID  uuid.UUID
	OrgID   uuid.UUID
	Email   string
	Role    Role
	TokenID string // jti — for denylist revocation checks
}

type contextKey struct{}

// WithAuthContext injects an AuthContext into the request context.
// Called only by auth middleware.
func WithAuthContext(ctx context.Context, ac *AuthContext) context.Context {
	return context.WithValue(ctx, contextKey{}, ac)
}

// GetAuthContext retrieves the AuthContext from context.
// Returns an error if no auth context is present.
func GetAuthContext(ctx context.Context) (*AuthContext, error) {
	ac, ok := ctx.Value(contextKey{}).(*AuthContext)
	if !ok || ac == nil {
		return nil, errors.New("no auth context in request")
	}
	return ac, nil
}
