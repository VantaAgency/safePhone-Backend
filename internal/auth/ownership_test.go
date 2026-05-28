package auth

import (
	"testing"

	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

func TestEnsureOwnership(t *testing.T) {
	orgA := uuid.New()
	orgB := uuid.New()
	userX := uuid.New()
	userY := uuid.New()

	tests := []struct {
		name          string
		ac            *AuthContext
		ownerOrgID    uuid.UUID
		ownerUserID   uuid.UUID
		wantCode      string // empty → expect nil error
		wantNilResult bool
	}{
		{
			name:        "owner same org and same user",
			ac:          &AuthContext{OrgID: orgA, UserID: userX, Role: RoleMember},
			ownerOrgID:  orgA,
			ownerUserID: userX,
		},
		{
			name:        "member in same org but different user is denied",
			ac:          &AuthContext{OrgID: orgA, UserID: userY, Role: RoleMember},
			ownerOrgID:  orgA,
			ownerUserID: userX,
			wantCode:    domain.CodeNotFound,
		},
		{
			name:        "different org always denied — admin role",
			ac:          &AuthContext{OrgID: orgB, UserID: userY, Role: RoleAdmin},
			ownerOrgID:  orgA,
			ownerUserID: userX,
			wantCode:    domain.CodeNotFound,
		},
		{
			name:        "admin in same org can access other user's resource",
			ac:          &AuthContext{OrgID: orgA, UserID: userY, Role: RoleAdmin},
			ownerOrgID:  orgA,
			ownerUserID: userX,
		},
		{
			name:        "employee in same org can access other user's resource",
			ac:          &AuthContext{OrgID: orgA, UserID: userY, Role: RoleEmployee},
			ownerOrgID:  orgA,
			ownerUserID: userX,
		},
		{
			name:        "commercial in same org can access other user's resource",
			ac:          &AuthContext{OrgID: orgA, UserID: userY, Role: RoleCommercial},
			ownerOrgID:  orgA,
			ownerUserID: userX,
		},
		{
			name:        "partner does NOT get cross-user access via this helper",
			ac:          &AuthContext{OrgID: orgA, UserID: userY, Role: RolePartner},
			ownerOrgID:  orgA,
			ownerUserID: userX,
			wantCode:    domain.CodeNotFound,
		},
		{
			name:        "org-scoped resource (no per-user owner) accepts any same-org user",
			ac:          &AuthContext{OrgID: orgA, UserID: userY, Role: RoleMember},
			ownerOrgID:  orgA,
			ownerUserID: uuid.Nil,
		},
		{
			name:        "nil AuthContext yields unauthorized",
			ac:          nil,
			ownerOrgID:  orgA,
			ownerUserID: userX,
			wantCode:    domain.CodeUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ac.EnsureOwnership(tt.ownerOrgID, tt.ownerUserID, "resource")
			if tt.wantCode == "" {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected error code %q, got nil", tt.wantCode)
			}
			if got.Code != tt.wantCode {
				t.Fatalf("expected error code %q, got %q (message %q)", tt.wantCode, got.Code, got.Message)
			}
		})
	}
}
