package contextgateway

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
	"github.com/DATA-DOG/go-sqlmock"
)

const (
	userID        = "11111111-1111-4111-8111-111111111111"
	projectID     = "22222222-2222-4222-8222-222222222222"
	threadID      = "33333333-3333-4333-8333-333333333333"
	artifactID    = "44444444-4444-4444-8444-444444444444"
	revisionID    = "55555555-5555-4555-8555-555555555555"
	otherThreadID = "66666666-6666-4666-8666-666666666666"
)

func scopeFixture() generation.Scope {
	return generation.Scope{UserID: userID, ProjectID: projectID, ThreadID: threadID}
}

func scopeRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "title", "description"}).AddRow(projectID, "Orders", "Project brief")
}

func sourceRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"artifact_id", "revision_id", "version", "title", "type", "format", "bytes", "body"})
}

func messageRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "thread_id", "role", "sequence", "bytes", "content"})
}

func expectScope(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT p.id, p.title, p.description FROM projects p`).WithArgs(projectID, userID, threadID).WillReturnRows(scopeRows())
}

func expectMessages(mock sqlmock.Sqlmock, thread string) *sqlmock.ExpectedQuery {
	return mock.ExpectQuery(`SELECT m.id, m.thread_id, m.role, m.sequence_number, octet_length\(m.content\)`).WithArgs(projectID, userID, thread, 1024, generation.MessagesPerThread+1)
}

func TestReadSnapshotPinsRequestedRevision(test *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		test.Fatal(err)
	}
	defer db.Close()
	expectScope(mock)
	expectMessages(mock, threadID).WillReturnRows(messageRows().AddRow("message", threadID, "user", 8, 5, "hello"))
	query := sourceSelect + ` AND a.id = $4 AND r.id = COALESCE(NULLIF($5, '')::uuid, a.current_revision_id)`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(projectID, userID, 1024, artifactID, revisionID).
		WillReturnRows(sourceRows().AddRow(artifactID, revisionID, 4, "PRD", "prd", "markdown", 19, []byte(`{"markdown": "PRD"}`)))
	mock.ExpectCommit()
	result, err := NewPostgresReader(db).ReadSnapshot(context.Background(), scopeFixture(), generation.ResolveInput{Mode: "explicit", MaxContentBytes: 1024, Sources: []generation.RevisionRef{{ArtifactID: artifactID, RevisionID: revisionID}}})
	if err != nil {
		test.Fatal(err)
	}
	if len(result.Sources) != 1 || result.Sources[0].RevisionID != revisionID || len(result.Messages) != 1 {
		test.Fatalf("unexpected result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		test.Fatal(err)
	}
}

func TestReadSnapshotRejectsUnavailableReferences(test *testing.T) {
	for _, scenario := range []string{"project or active thread", "selected thread", "source or mismatched revision"} {
		test.Run(scenario, func(test *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				test.Fatal(err)
			}
			defer db.Close()
			input := generation.ResolveInput{Mode: "explicit", MaxContentBytes: 1024}
			if scenario == "project or active thread" {
				mock.ExpectBegin()
				mock.ExpectQuery(`SELECT p.id, p.title, p.description FROM projects p`).WithArgs(projectID, userID, threadID).WillReturnError(sql.ErrNoRows)
			} else {
				expectScope(mock)
				expectMessages(mock, threadID).WillReturnRows(messageRows())
				if scenario == "selected thread" {
					input.ThreadIDs = []string{otherThreadID}
					mock.ExpectQuery(`SELECT id FROM chat_threads WHERE id = \$1 AND project_id = \$2`).WithArgs(otherThreadID, projectID).WillReturnError(sql.ErrNoRows)
				} else {
					input.Sources = []generation.RevisionRef{{ArtifactID: artifactID, RevisionID: revisionID}}
					mock.ExpectQuery(regexp.QuoteMeta(sourceSelect)).WithArgs(projectID, userID, 1024, artifactID, revisionID).WillReturnError(sql.ErrNoRows)
				}
			}
			mock.ExpectRollback()
			_, err = NewPostgresReader(db).ReadSnapshot(context.Background(), scopeFixture(), input)
			if !errors.Is(err, generation.ErrNotFound) {
				test.Fatalf("got %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				test.Fatal(err)
			}
		})
	}
}

func TestAutomaticSnapshotLimitsAndExclusions(test *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		test.Fatal(err)
	}
	defer db.Close()
	expectScope(mock)
	messages := messageRows()
	for sequence := generation.MessagesPerThread + 1; sequence > 0; sequence-- {
		messages.AddRow("message", threadID, "assistant", sequence, 5, "hello")
	}
	expectMessages(mock, threadID).WillReturnRows(messages)
	mock.ExpectQuery(`SELECT id FROM chat_threads`).WithArgs(otherThreadID, projectID).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(otherThreadID))
	expectMessages(mock, otherThreadID).WillReturnRows(messageRows())
	rows := sourceRows()
	for index := 0; index <= generation.CandidateLimit; index++ {
		rows.AddRow(artifactID, revisionID, 1, "PRD", "prd", "markdown", 2000, nil)
	}
	query := sourceSelect + ` AND r.id = a.current_revision_id AND NOT (a.id = ANY(COALESCE($4::uuid[], '{}'::uuid[]))) ORDER BY a.updated_at DESC, a.id DESC LIMIT $5`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(projectID, userID, 1024, `{"`+artifactID+`"}`, generation.CandidateLimit+1).WillReturnRows(rows)
	mock.ExpectCommit()
	result, err := NewPostgresReader(db).ReadSnapshot(context.Background(), scopeFixture(), generation.ResolveInput{Mode: "project_relevant", MaxContentBytes: 1024, ExcludedArtifactIDs: []string{artifactID}, ThreadIDs: []string{otherThreadID}})
	if err != nil {
		test.Fatal(err)
	}
	if !result.CandidateLimitReached || len(result.Sources) != generation.CandidateLimit || len(result.Messages) != generation.MessagesPerThread || len(result.HistoryLimitedThreadIDs) != 1 {
		test.Fatalf("limits not reported: %+v", result)
	}
	if len(result.Sources[0].Content) != 0 {
		test.Fatal("oversized source should not be fetched")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		test.Fatal(err)
	}
}

func TestSnapshotErrorsDoNotReturnPartialContext(test *testing.T) {
	for _, scenario := range []string{"message read", "commit"} {
		test.Run(scenario, func(test *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				test.Fatal(err)
			}
			defer db.Close()
			expectScope(mock)
			if scenario == "message read" {
				expectMessages(mock, threadID).WillReturnError(errors.New("query failed"))
				mock.ExpectRollback()
			} else {
				expectMessages(mock, threadID).WillReturnRows(messageRows())
				mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
			}
			result, err := NewPostgresReader(db).ReadSnapshot(context.Background(), scopeFixture(), generation.ResolveInput{Mode: "explicit", MaxContentBytes: 1024})
			if err == nil || result != nil {
				test.Fatalf("partial result=%+v error=%v", result, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				test.Fatal(err)
			}
		})
	}
}
