package projectgateway

import (
	"context"
	"database/sql"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/repository"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing"
	shareservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing/service"
)

type SQLProjectGateway struct {
	repository *repository.PostgresCloneRepository
}

func NewSQLProjectGateway(db *sql.DB) shareservice.ProjectGateway {
	return &SQLProjectGateway{repository: repository.NewCloneRepository(db)}
}

func (gateway *SQLProjectGateway) LoadSharedProject(ctx context.Context, projectID string) (*sharing.Project, []sharing.Message, error) {
	project, messages, err := gateway.repository.LoadProjectForClone(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	result := sharing.Project(*project)
	var mapped []sharing.Message
	if messages != nil {
		mapped = make([]sharing.Message, len(messages))
		for index, message := range messages {
			mapped[index] = sharing.Message(message)
		}
	}
	return &result, mapped, nil
}

func (gateway *SQLProjectGateway) CloneSharedProject(ctx context.Context, input sharing.CloneInput) (*sharing.Project, error) {
	var messages []diagram.ChatMessage
	if input.Messages != nil {
		messages = make([]diagram.ChatMessage, len(input.Messages))
		for index, message := range input.Messages {
			messages[index] = diagram.ChatMessage(message)
		}
	}
	project, err := gateway.repository.CloneProject(ctx, diagram.CloneInput{
		Original: diagram.Project(input.Original), Messages: messages, Title: input.Title, UserID: input.UserID,
	})
	if err != nil {
		return nil, err
	}
	result := sharing.Project(*project)
	return &result, nil
}
