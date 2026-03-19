package auth

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwk"

	"github.com/cherif-safephone/safephone-backend/internal/cache"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/respond"
)

// JWTVerifier handles ES256 JWT verification using JWKS.
type JWTVerifier struct {
	jwksURL   string
	issuer    string
	cache     *cache.Client
	keySet    jwk.Set
	keyMu     sync.RWMutex
	lastFetch time.Time
	cacheTTL  time.Duration
}

// NewJWTVerifier creates a new JWT verifier that fetches public keys from the JWKS endpoint.
func NewJWTVerifier(jwksURL, issuer string, redisCache *cache.Client) *JWTVerifier {
	return &JWTVerifier{
		jwksURL:  jwksURL,
		issuer:   issuer,
		cache:    redisCache,
		cacheTTL: 5 * time.Minute,
	}
}

// fetchKeys fetches and caches the JWKS key set.
func (v *JWTVerifier) fetchKeys(ctx context.Context) (jwk.Set, error) {
	v.keyMu.RLock()
	if v.keySet != nil && time.Since(v.lastFetch) < v.cacheTTL {
		defer v.keyMu.RUnlock()
		return v.keySet, nil
	}
	v.keyMu.RUnlock()

	v.keyMu.Lock()
	defer v.keyMu.Unlock()

	// Double-check after acquiring write lock
	if v.keySet != nil && time.Since(v.lastFetch) < v.cacheTTL {
		return v.keySet, nil
	}

	set, err := jwk.Fetch(ctx, v.jwksURL)
	if err != nil {
		slog.Warn("failed to fetch JWKS", "jwks_url", v.jwksURL, "error", err)
		// If we have a cached set, use it even if stale
		if v.keySet != nil {
			return v.keySet, nil
		}
		return nil, fmt.Errorf("fetching JWKS: %w", err)
	}

	v.keySet = set
	v.lastFetch = time.Now()
	return set, nil
}

// getPublicKey retrieves the ECDSA public key from the JWKS by kid.
func (v *JWTVerifier) getPublicKey(ctx context.Context, kid string) (*ecdsa.PublicKey, error) {
	set, err := v.fetchKeys(ctx)
	if err != nil {
		return nil, err
	}

	key, found := set.LookupKeyID(kid)
	if !found {
		return nil, fmt.Errorf("key %q not found in JWKS", kid)
	}

	var pubKey ecdsa.PublicKey
	if err := key.Raw(&pubKey); err != nil {
		return nil, fmt.Errorf("extracting public key: %w", err)
	}

	return &pubKey, nil
}

// Authenticate returns middleware that verifies ES256 JWTs and injects AuthContext.
func (v *JWTVerifier) Authenticate(next http.Handler) http.Handler {
	return v.authenticate(next, false)
}

// AuthenticateOptional verifies JWTs when present and otherwise proceeds unauthenticated.
func (v *JWTVerifier) AuthenticateOptional(next http.Handler) http.Handler {
	return v.authenticate(next, true)
}

func (v *JWTVerifier) authenticate(next http.Handler, optional bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			if optional {
				next.ServeHTTP(w, r)
				return
			}
			respond.Error(w, r, domain.Unauthorized("missing authorization header"))
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			respond.Error(w, r, domain.Unauthorized("invalid authorization format"))
			return
		}

		// Parse and verify the token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			// Enforce ES256 algorithm — reject all others
			if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			kid, ok := token.Header["kid"].(string)
			if !ok {
				return nil, fmt.Errorf("missing kid in token header")
			}

			return v.getPublicKey(r.Context(), kid)
		}, jwt.WithValidMethods([]string{"ES256"}))

		if err != nil || !token.Valid {
			if err != nil {
				slog.Warn("jwt verification failed", "error", err)
			} else {
				slog.Warn("jwt verification failed", "error", "token marked invalid")
			}
			respond.Error(w, r, domain.Unauthorized("invalid or expired token"))
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			respond.Error(w, r, domain.Unauthorized("invalid token claims"))
			return
		}

		// Extract claims
		jti, _ := claims["jti"].(string)

		// Check Redis denylist for revoked tokens
		if jti != "" && v.cache != nil {
			revoked, err := v.cache.Exists(r.Context(), "jwt:revoked:"+jti)
			if err != nil {
				// Fail closed in production — reject token if Redis is unavailable
				respond.Error(w, r, domain.InternalError(fmt.Errorf("checking token denylist: %w", err)))
				return
			}
			if revoked {
				respond.Error(w, r, domain.Unauthorized("token has been revoked"))
				return
			}
		}

		// Build AuthContext from claims
		sub, _ := claims["sub"].(string)
		userID, err := uuid.Parse(sub)
		if err != nil {
			respond.Error(w, r, domain.Unauthorized("invalid user ID in token"))
			return
		}

		orgIDStr, _ := claims["org_id"].(string)
		orgID, err := uuid.Parse(orgIDStr)
		if err != nil {
			respond.Error(w, r, domain.Unauthorized("invalid org ID in token"))
			return
		}

		email, _ := claims["email"].(string)
		role, _ := claims["role"].(string)

		ac := &AuthContext{
			UserID:  userID,
			OrgID:   orgID,
			Email:   email,
			Role:    Role(role),
			TokenID: jti,
		}

		ctx := WithAuthContext(r.Context(), ac)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
