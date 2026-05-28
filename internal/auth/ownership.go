package auth

import (
	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// CanCrossUser is the set of roles allowed to read/modify resources that
// belong to other users within the same org. Members are restricted to their
// own resources; admins, employees and commercials act on behalf of the org
// and routinely need cross-user access.
//
// Partners are intentionally NOT here — they get their own scoped repository
// access via partner-specific routes; granting cross-user via this helper
// would silently widen their reach beyond their partner scope.
var crossUserRoles = []Role{RoleAdmin, RoleEmployee, RoleCommercial}

// CanCrossUser reports whether the auth context allows accessing resources
// owned by another user in the same org.
func (ac *AuthContext) CanCrossUser() bool {
	if ac == nil {
		return false
	}
	for _, r := range crossUserRoles {
		if ac.HasRole(r) {
			return true
		}
	}
	return false
}

// EnsureOwnership returns nil when the authenticated user is allowed to
// access a resource owned by (ownerOrgID, ownerUserID). Otherwise it returns
// a NotFound — never a Forbidden — so we don't leak resource existence to
// users who happen to know its ID.
//
// Rules:
//   - Different org           → NotFound (the resource is invisible).
//   - Same org, same user     → allowed.
//   - Same org, other user    → allowed only for admin/employee/commercial roles.
//
// Pass uuid.Nil for ownerUserID when the resource is org-scoped without a
// per-user owner (e.g. partner profile).
func (ac *AuthContext) EnsureOwnership(ownerOrgID, ownerUserID uuid.UUID, resourceLabel string) *domain.AppError {
	if ac == nil {
		return domain.Unauthorized("authentication required")
	}
	if ac.OrgID != ownerOrgID {
		return domain.NotFound(resourceLabel)
	}
	if ownerUserID == uuid.Nil || ac.UserID == ownerUserID {
		return nil
	}
	if ac.CanCrossUser() {
		return nil
	}
	// Same org, different user, no cross-user role → look the same as "not
	// found" to the caller (prevents user enumeration via ID guessing).
	return domain.NotFound(resourceLabel)
}
