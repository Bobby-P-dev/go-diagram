package authcontext

import (
	"context"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
)

func TestWithUserAndUserFromContext(t *testing.T) {
	if UserFromContext(nil) != nil {
		t.Fatal("expected nil for nil context")
	}

	bg := context.Background()
	if UserFromContext(bg) != nil {
		t.Fatal("expected nil when no user in context")
	}

	user := &service.CredentialRecord{
		ID:   "usr-1",
		Name: "Test User",
		Role: "admin",
	}

	ctx := WithUser(bg, user)
	retrieved := UserFromContext(ctx)
	if retrieved == nil {
		t.Fatal("expected retrieved user, got nil")
	}
	if retrieved.ID != "usr-1" || retrieved.Name != "Test User" || retrieved.Role != "admin" {
		t.Fatalf("unexpected user retrieved: %+v", retrieved)
	}
}
