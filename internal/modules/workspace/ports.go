package workspace

import "context"

type Actor struct {
	ID   string
	Role string
}

type ProjectAccessReader interface {
	CheckProjectAccess(ctx context.Context, projectID, userID, role string) error
}
