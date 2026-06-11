package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// A token crafted to climb out of the store root (e.g. via a non-UUID userID
// segment containing "..") must be rejected before any file is opened, so a
// caller can't read arbitrary files off the host.
func TestLocalVerificationMediaStoreRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	store := NewLocalVerificationMediaStore(root)

	// Plant a secret OUTSIDE the store root.
	secretDir := t.TempDir()
	secretPath := filepath.Join(secretDir, "secret.txt")
	if err := os.WriteFile(secretPath, []byte("top secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	rel, err := filepath.Rel(root, secretPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Open(context.Background(), filepath.ToSlash(rel)); err == nil {
		t.Fatal("expected traversal token to be rejected, got nil error")
	}

	// A legitimately stored object still opens fine.
	token, err := store.Store(context.Background(), "11111111-1111-1111-1111-111111111111", ".txt", "text/plain", []byte("hi"))
	if err != nil {
		t.Fatalf("store failed: %v", err)
	}
	rc, _, err := store.Open(context.Background(), token)
	if err != nil {
		t.Fatalf("legit open failed: %v", err)
	}
	_ = rc.Close()
}
