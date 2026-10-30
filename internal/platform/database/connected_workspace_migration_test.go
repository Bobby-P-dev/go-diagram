package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/lib/pq"
)

func workspaceTestCluster(test *testing.T) func() *sql.DB {
	test.Helper()
	if os.Getenv("RUN_DATABASE_INTEGRATION_TESTS") != "1" {
		test.Skip("set RUN_DATABASE_INTEGRATION_TESTS=1 to start a disposable local PostgreSQL cluster")
	}
	if os.Geteuid() == 0 {
		test.Fatal("run database integration tests as a non-root user")
	}
	binDir := os.Getenv("POSTGRES_BIN")
	if binDir == "" {
		binDir = "/usr/lib/postgresql/16/bin"
	}
	for _, binary := range []string{"initdb", "pg_ctl"} {
		if _, err := os.Stat(filepath.Join(binDir, binary)); err != nil {
			test.Fatalf("PostgreSQL binary unavailable: %v; set POSTGRES_BIN", err)
		}
	}
	directory, err := os.MkdirTemp("", "workspace-pg-")
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { _ = os.RemoveAll(directory) })
	dataDir := filepath.Join(directory, "data")
	run := func(arguments ...string) {
		test.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		output, err := exec.CommandContext(ctx, filepath.Join(binDir, arguments[0]), arguments[1:]...).CombinedOutput()
		if err != nil {
			logs, _ := os.ReadFile(filepath.Join(directory, "postgres.log"))
			test.Fatalf("%s failed: %v\n%s\n%s", arguments[0], err, output, logs)
		}
	}
	run("initdb", "-D", dataDir, "-A", "trust", "-U", "postgres", "--no-sync", "--no-locale", "--encoding=UTF8")
	test.Cleanup(func() { run("pg_ctl", "-D", dataDir, "-m", "immediate", "-w", "stop") })
	run("pg_ctl", "-D", dataDir, "-l", filepath.Join(directory, "postgres.log"),
		"-o", fmt.Sprintf("-F -h '' -k '%s' -p 55439", directory), "-w", "start")
	open := func(name string) *sql.DB {
		test.Helper()
		db, err := sql.Open("postgres", fmt.Sprintf("host=%s port=55439 user=postgres dbname=%s sslmode=disable", directory, name))
		if err != nil {
			test.Fatal(err)
		}
		test.Cleanup(func() { _ = db.Close() })
		if err := db.Ping(); err != nil {
			test.Fatal(err)
		}
		return db
	}
	admin := open("postgres")
	fixtureNumber := 0
	return func() *sql.DB {
		fixtureNumber++
		name := fmt.Sprintf("workspace_fixture_%d", fixtureNumber)
		if _, err := admin.Exec("CREATE DATABASE " + pq.QuoteIdentifier(name)); err != nil {
			test.Fatal(err)
		}
		return open(name)
	}
}

func executeFixture(test *testing.T, db *sql.DB, query string, arguments ...any) {
	test.Helper()
	if _, err := db.Exec(query, arguments...); err != nil {
		test.Fatal(err)
	}
}

func assertCount(test *testing.T, db *sql.DB, expected int, query string, arguments ...any) {
	test.Helper()
	var count int
	if err := db.QueryRow(query, arguments...).Scan(&count); err != nil || count != expected {
		test.Fatalf("count: got %d, want %d, error=%v; %s", count, expected, err, query)
	}
}

func assertSQLState(test *testing.T, err error, expected string) {
	test.Helper()
	var postgresError *pq.Error
	if !errors.As(err, &postgresError) || string(postgresError.Code) != expected {
		test.Fatalf("expected SQLSTATE %s, got %v", expected, err)
	}
}

func createFixtureArtifact(test *testing.T, db *sql.DB, projectID, slug string) (string, string) {
	test.Helper()
	tx, err := db.Begin()
	if err != nil {
		test.Fatal(err)
	}
	defer tx.Rollback()
	var artifactID, revisionID string
	err = tx.QueryRow(`INSERT INTO artifacts(project_id, type, title, slug, format)
		VALUES ($1, 'erd', 'Database model', $2, 'graph') RETURNING id, current_revision_id`, projectID, slug).Scan(&artifactID, &revisionID)
	if err != nil {
		test.Fatal(err)
	}
	_, err = tx.Exec(`INSERT INTO artifact_revisions(id, project_id, artifact_id, version_number,
		content_format, graph_nodes, graph_edges, author_type) VALUES ($1,$2,$3,1,'graph','[]','[]','manual')`, revisionID, projectID, artifactID)
	if err != nil {
		test.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		test.Fatal(err)
	}
	return artifactID, revisionID
}

func TestConnectedWorkspaceIntegration(test *testing.T) {
	newFixture := workspaceTestCluster(test)
	ctx := context.Background()
	test.Run("fresh_and_repeat", func(test *testing.T) {
		db := newFixture()
		for iteration := 0; iteration < 2; iteration++ {
			if err := RunMigrations(ctx, db); err != nil {
				test.Fatal(err)
			}
		}
		assertCount(test, db, 5, "SELECT count(*) FROM schema_migrations")
		assertCount(test, db, 0, "SELECT count(*) FROM legacy_workspace_imports")
		for _, table := range []string{"chat_threads", "chat_messages_v2", "artifacts", "artifact_revisions", "artifact_context_links", "product_decisions", "idempotency_keys"} {
			assertCount(test, db, 1, "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name=$1", table)
		}
	})
	test.Run("legacy_history_and_idempotent_backfill", func(test *testing.T) {
		db := newFixture()
		executeFixture(test, db, initSQL)
		executeFixture(test, db, `INSERT INTO access_credentials(id,credential_key,name) VALUES
			('10000000-0000-4000-8000-000000000001','fixture-only','Fixture owner');
			INSERT INTO projects(id,user_id,title,diagram_type,project_mode,current_nodes) VALUES
			('20000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001','ERD','erd','diagram','[{"id":"current"}]'),
			('20000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000001','UI','ui_design','ui_design','[{"id":"frame","type":"ui_frame","data":{"raw_html":"<footer>Keep me</footer>"}}]'),
			('20000000-0000-4000-8000-000000000003',NULL,'Ownerless','flowchart','diagram','[]');
			INSERT INTO chat_messages(id,project_id,role,content,target_node_ids,metadata) VALUES
			('30000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001','user','Keep conversation','["orders"]','{"note":"keep"}');
			INSERT INTO diagram_versions(id,project_id,version_number,nodes,edges,trigger_message_id) VALUES
			('40000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001',1,'[{"id":"old1"}]','[]','30000000-0000-4000-8000-000000000001'),
			('40000000-0000-4000-8000-000000000002','20000000-0000-4000-8000-000000000001',1,'[{"id":"old2"}]','[]',NULL);`)
		if err := RunMigrations(ctx, db); err != nil {
			test.Fatal(err)
		}
		assertCount(test, db, 2, "SELECT count(*) FROM legacy_workspace_imports")
		assertCount(test, db, 1, "SELECT count(*) FROM chat_messages_v2 WHERE content='Keep conversation' AND target_element_ids='[\"orders\"]' AND metadata->>'note'='keep'")
		assertCount(test, db, 3, "SELECT count(*) FROM artifact_revisions WHERE content_format='graph'")
		assertCount(test, db, 2, "SELECT count(*) FROM artifact_revisions WHERE metadata->>'legacy_version_number'='1'")
		assertCount(test, db, 1, "SELECT count(*) FROM artifacts WHERE type='erd' AND current_version_number=3")
		assertCount(test, db, 1, "SELECT count(*) FROM artifact_revisions WHERE graph_nodes='[{\"id\":\"current\"}]'")
		assertCount(test, db, 1, "SELECT count(*) FROM artifact_revisions r JOIN projects p ON r.project_id=p.id WHERE r.content_format='frame' AND r.frame_data->'nodes'=p.current_nodes AND r.frame_data->'edges'=p.current_edges")
		executeFixture(test, db, "UPDATE projects SET user_id='10000000-0000-4000-8000-000000000001' WHERE user_id IS NULL")
		if err := BackfillConnectedWorkspace(ctx, db); err != nil {
			test.Fatal(err)
		}
		if err := RunMigrations(ctx, db); err != nil {
			test.Fatal(err)
		}
		assertCount(test, db, 3, "SELECT count(*) FROM legacy_workspace_imports")
		assertCount(test, db, 5, "SELECT count(*) FROM artifact_revisions")
		assertCount(test, db, 1, "SELECT count(*) FROM chat_messages_v2")
		assertCount(test, db, 2, "SELECT count(*) FROM diagram_versions")
		assertCount(test, db, 1, "SELECT count(*) FROM access_credentials WHERE credential_key='fixture-only'")
	})
	test.Run("composite_constraints_and_immutability", func(test *testing.T) {
		db := newFixture()
		if err := RunMigrations(ctx, db); err != nil {
			test.Fatal(err)
		}
		const projectA = "20000000-0000-4000-8000-000000000001"
		const projectB = "20000000-0000-4000-8000-000000000002"
		executeFixture(test, db, `INSERT INTO projects(id,title,diagram_type,project_mode) VALUES
			($1,'Project A','none','workspace'),($2,'Project B','none','workspace')`, projectA, projectB)
		artifactA, revisionA := createFixtureArtifact(test, db, projectA, "erd-a")
		artifactA2, _ := createFixtureArtifact(test, db, projectA, "erd-a2")
		artifactB, revisionB := createFixtureArtifact(test, db, projectB, "erd-b")
		var threadID string
		if err := db.QueryRow("INSERT INTO chat_threads(project_id,title) VALUES ($1,'Thread') RETURNING id", projectA).Scan(&threadID); err != nil {
			test.Fatal(err)
		}
		_, err := db.Exec("INSERT INTO chat_messages_v2(project_id,thread_id,role,content) VALUES ($1,$2,'user','Invalid')", projectB, threadID)
		assertSQLState(test, err, "23503")
		_, err = db.Exec(`INSERT INTO artifact_revisions(project_id,artifact_id,version_number,content_format,graph_nodes,graph_edges,author_type)
			VALUES ($1,$2,2,'graph','[]','[]','manual')`, projectB, artifactA)
		assertSQLState(test, err, "23503")
		_, err = db.Exec(`INSERT INTO artifact_revisions(project_id,artifact_id,version_number,content_format,graph_nodes,graph_edges,author_type)
			VALUES ($1,$2,1,'graph','[]','[]','manual')`, projectA, artifactA)
		assertSQLState(test, err, "23505")
		_, err = db.Exec("UPDATE artifact_revisions SET change_summary='overwrite' WHERE id=$1", revisionA)
		assertSQLState(test, err, "23514")
		_, err = db.Exec("DELETE FROM artifact_revisions WHERE id=$1", revisionA)
		assertSQLState(test, err, "23514")
		_, err = db.Exec("UPDATE artifacts SET current_revision_id=$1 WHERE id=$2", revisionB, artifactA)
		assertSQLState(test, err, "23503")
		_, err = db.Exec("UPDATE chat_threads SET active_artifact_id=$1 WHERE id=$2", artifactB, threadID)
		assertSQLState(test, err, "23503")
		for _, upstream := range []struct{ artifact, revision string }{{artifactB, revisionB}, {artifactA2, revisionA}} {
			_, err = db.Exec(`INSERT INTO artifact_context_links(project_id,downstream_artifact_id,downstream_revision_number,upstream_artifact_id,upstream_revision_id)
				VALUES ($1,$2,1,$3,$4)`, projectA, artifactA, upstream.artifact, upstream.revision)
			assertSQLState(test, err, "23503")
		}
		_, err = db.Exec("INSERT INTO product_decisions(project_id,title,source_artifact_id) VALUES ($1,'Invalid source',$2)", projectA, artifactB)
		assertSQLState(test, err, "23503")
		executeFixture(test, db, "INSERT INTO product_decisions(project_id,title) VALUES ($1,'Not approved automatically')", projectA)
		assertCount(test, db, 1, "SELECT count(*) FROM product_decisions WHERE status='proposed'")
		executeFixture(test, db, "DELETE FROM projects WHERE id=$1", projectB)
		assertCount(test, db, 0, "SELECT count(*) FROM artifact_revisions WHERE project_id=$1", projectB)
	})
	test.Run("atomic_failure_and_checksum_drift", func(test *testing.T) {
		db := newFixture()
		executeFixture(test, db, "CREATE TABLE chat_threads(conflict INT)")
		if err := RunMigrations(ctx, db); err == nil {
			test.Fatal("expected conflicting table to fail migration")
		}
		assertCount(test, db, 0, "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('projects','schema_migrations','artifacts')")
		executeFixture(test, db, "DROP TABLE chat_threads")
		if err := RunMigrations(ctx, db); err != nil {
			test.Fatal(err)
		}
		executeFixture(test, db, "UPDATE schema_migrations SET checksum='tampered'")
		if err := RunMigrations(ctx, db); err == nil {
			test.Fatal("expected checksum drift rejection")
		}
	})
}
