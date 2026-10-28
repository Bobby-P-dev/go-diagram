package aiusage_test

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/aiusage"
	"github.com/DATA-DOG/go-sqlmock"
)

var _ aiusage.Recorder = (*aiusage.PostgresRepository)(nil)

type recordedAt struct{ start time.Time }

func (argument recordedAt) Match(value driver.Value) bool {
	created, ok := value.(time.Time)
	return ok && !created.Before(argument.start) && !created.After(time.Now())
}

func TestAIUsageInsert(t *testing.T) {
	projectID := "project"
	for _, id := range []*string{nil, &projectID} {
		t.Run(map[bool]string{true: "without project", false: "with project"}[id == nil], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			entry := aiusage.Entry{ProjectID: id, Provider: "provider", Model: "model", PromptTokens: 12, CompletionTokens: 3, TotalTokens: 15, LatencyMS: 100, CreatedAt: time.Unix(1, 0)}
			mock.ExpectExec(`INSERT INTO ai_usage_logs`).WithArgs(id, "provider", "model", 12, 3, 15, 100, recordedAt{start: time.Now()}).WillReturnResult(sqlmock.NewResult(0, 1))
			if err := aiusage.NewPostgresRepository(db).LogAIUsage(context.Background(), entry); err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAIUsageErrorsAndContext(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := aiusage.NewPostgresRepository(db)
	sentinel := errors.New("database unavailable")
	mock.ExpectExec(`INSERT INTO ai_usage_logs`).WillReturnError(sentinel)
	if err := repo.LogAIUsage(context.Background(), aiusage.Entry{}); !errors.Is(err, sentinel) {
		t.Fatalf("insert error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := repo.LogAIUsage(ctx, aiusage.Entry{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
