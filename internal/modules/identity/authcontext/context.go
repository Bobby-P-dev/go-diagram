package authcontext

import (
	"context"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
)

type userKey struct{}

func WithUser(ctx context.Context, user *service.CredentialRecord) context.Context {
	return context.WithValue(ctx, userKey{}, user)
}

func UserFromContext(ctx context.Context) *service.CredentialRecord {
	if ctx == nil {
		return nil
	}
	user, _ := ctx.Value(userKey{}).(*service.CredentialRecord)
	return user
}
