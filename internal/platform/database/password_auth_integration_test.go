package database

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Bobby-P-dev/go-diagram.git/internal/app"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity"
	identityrepo "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/repository"
	identityservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/config"
)

func TestPasswordAuthIntegration(test *testing.T) {
	newFixture := workspaceTestCluster(test)
	db := newFixture()
	ctx := context.Background()
	if err := RunMigrations(ctx, db); err != nil {
		test.Fatal(err)
	}
	password := "private test password with spaces "
	hash, err := identity.HashPassword(password)
	if err != nil {
		test.Fatal(err)
	}
	userID, err := identityrepo.ProvisionPasswordAccount(ctx, db, identityrepo.ProvisionAccount{Email: "developer@example.test", Name: "Developer", PasswordHash: hash, DisabledKeyHash: identity.HashCredentialKey("disabled-test-token")})
	if err != nil {
		test.Fatal(err)
	}
	if _, err = identityrepo.ProvisionPasswordAccount(ctx, db, identityrepo.ProvisionAccount{Email: "developer@example.test", Name: "Duplicate", PasswordHash: hash, DisabledKeyHash: identity.HashCredentialKey("second-key")}); err == nil {
		test.Fatal("duplicate provisioning must not overwrite")
	}
	assertCount(test, db, 1, `SELECT count(*) FROM access_credentials`)
	credentials := identityservice.NewCredentialService(identityrepo.NewPostgresRepository(db))
	handler := app.NewHandler(&config.EnvConfig{AIProvider: "openai", InternalServiceKey: "test-internal-key", CORSAllowedOrigins: "http://localhost:5174", AuthCookieSecure: true}, db, nil, credentials)
	request := func(method, path, body string, cookie *http.Cookie, csrf, origin string) *httptest.ResponseRecorder {
		test.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	expect := func(response *httptest.ResponseRecorder, status int) {
		test.Helper()
		if response.Code != status {
			test.Fatalf("status %d want %d: %s", response.Code, status, response.Body.String())
		}
	}
	login := func(email, pass string, cookie *http.Cookie) *httptest.ResponseRecorder {
		payload, _ := json.Marshal(map[string]string{"email": email, "password": pass})
		return request("POST", "/api/auth/login", string(payload), cookie, "", "http://localhost:5174")
	}
	response := login("developer@example.test", "wrong", nil)
	expect(response, 401)
	unknown := login("unknown@example.test", "wrong", nil)
	expect(unknown, 401)
	if unknown.Body.String() != response.Body.String() {
		test.Fatal("login enumerates emails")
	}
	expect(login("developer@example.test", strings.TrimSpace(password), nil), 401)
	expect(request("GET", "/api/v2/projects", "", nil, "", ""), 401)
	expect(request("POST", "/api/auth/login", `{"email":"developer@example.test","password":"test","role":"admin"}`, nil, "", ""), 400)
	expect(request("POST", "/api/auth/login", `{} {}`, nil, "", ""), 400)
	expect(request("POST", "/api/auth/login", strings.Repeat("x", 5000), nil, "", ""), 400)
	formRequest := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader("email=developer%40example.test&password=test"))
	formRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	formResponse := httptest.NewRecorder()
	handler.ServeHTTP(formResponse, formRequest)
	expect(formResponse, 415)
	bearerRequest := httptest.NewRequest("GET", "/api/v2/projects", nil)
	bearerRequest.Header.Set("Authorization", "Bearer disabled-test-token")
	bearerResponse := httptest.NewRecorder()
	handler.ServeHTTP(bearerResponse, bearerRequest)
	expect(bearerResponse, 401)
	expect(request("POST", "/api/auth/verify", `{"credential_key":"disabled-test-token"}`, nil, "", ""), 404)
	expect(request("POST", "/api/auth/login", `{"email":"developer@example.test","password":"anything"}`, nil, "", "https://attacker.test"), 403)
	response = login(" Developer@Example.test ", password, nil)
	expect(response, 200)
	var session struct {
		Data struct {
			CSRF string `json:"csrf_token"`
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		test.Fatal(err)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		test.Fatal("expected one session cookie")
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.MaxAge != 43200 || session.Data.User.ID != userID || len(session.Data.CSRF) != 64 {
		test.Fatal("invalid session response or cookie")
	}
	if bytes.Contains(response.Body.Bytes(), []byte(cookie.Value)) || bytes.Contains(response.Body.Bytes(), []byte(hash)) || bytes.Contains(response.Body.Bytes(), []byte("credential_key")) {
		test.Fatal("response leaked secrets")
	}
	assertCount(test, db, 1, `SELECT count(*) FROM browser_sessions WHERE token_hash=$1`, identityservice.SessionHash(cookie.Value))
	assertCount(test, db, 0, `SELECT count(*) FROM browser_sessions WHERE token_hash=$1`, cookie.Value)
	expect(request("GET", "/api/auth/session", "", cookie, "", ""), 200)
	expect(request("GET", "/api/v2/projects", "", cookie, "", ""), 200)
	expect(request("POST", "/api/v2/projects", `{"title":"Demo"}`, cookie, "", "http://localhost:5174"), 403)
	expect(request("DELETE", "/api/auth/session", "", cookie, session.Data.CSRF, "https://attacker.test"), 403)
	expect(request("DELETE", "/api/auth/session", "", cookie, "wrong-token", "http://localhost:5174"), 403)
	write := httptest.NewRequest("POST", "/api/v2/projects", strings.NewReader(`{"title":"Password session project"}`))
	write.Header.Set("Content-Type", "application/json")
	write.Header.Set("X-CSRF-Token", session.Data.CSRF)
	write.Header.Set("Idempotency-Key", "password-project-1")
	write.AddCookie(cookie)
	written := httptest.NewRecorder()
	handler.ServeHTTP(written, write)
	expect(written, 201)
	assertCount(test, db, 1, `SELECT count(*) FROM projects WHERE user_id=$1`, userID)
	rotated := login("developer@example.test", password, cookie)
	expect(rotated, 200)
	expect(request("GET", "/api/auth/session", "", cookie, "", ""), 401)
	cookie = rotated.Result().Cookies()[0]
	if err = json.Unmarshal(rotated.Body.Bytes(), &session); err != nil {
		test.Fatal(err)
	}
	expect(request("POST", "/api/auth/logout", "", cookie, session.Data.CSRF, ""), 200)
	expect(request("GET", "/api/auth/session", "", cookie, "", ""), 401)
	assertCount(test, db, 0, `SELECT count(*) FROM browser_sessions`)
	response = login("developer@example.test", password, nil)
	expect(response, 200)
	cookie = response.Result().Cookies()[0]
	if err = json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		test.Fatal(err)
	}
	deleted := request("DELETE", "/api/auth/session", "", cookie, session.Data.CSRF, "http://localhost:5174")
	expect(deleted, 200)
	if cleared := deleted.Result().Cookies(); len(cleared) != 1 || cleared[0].MaxAge != -1 || cleared[0].Value != "" {
		test.Fatal("logout did not expire cookie")
	}
	expect(request("GET", "/api/auth/session", "", cookie, "", ""), 401)
	executeFixture(test, db, `UPDATE password_login_attempts SET window_started=NOW()-INTERVAL '16 minutes'`)
	response = login("developer@example.test", password, nil)
	expect(response, 200)
	cookie = response.Result().Cookies()[0]
	executeFixture(test, db, `UPDATE browser_sessions SET expires_at=NOW()-INTERVAL '1 second'`)
	expect(request("GET", "/api/auth/session", "", cookie, "", ""), 401)
	executeFixture(test, db, `UPDATE password_login_attempts SET window_started=NOW()-INTERVAL '16 minutes'`)
	response = login("developer@example.test", password, nil)
	expect(response, 200)
	cookie = response.Result().Cookies()[0]
	executeFixture(test, db, `UPDATE access_credentials SET is_active=false WHERE id=$1`, userID)
	expect(request("GET", "/api/v2/projects", "", cookie, "", ""), 401)
	expect(login("developer@example.test", password, nil), 401)
	executeFixture(test, db, `UPDATE access_credentials SET is_active=true,expires_at=NOW()-INTERVAL '1 second' WHERE id=$1`, userID)
	expect(login("developer@example.test", password, nil), 401)
	executeFixture(test, db, `UPDATE password_login_attempts SET attempts=10,window_started=NOW() WHERE bucket=$1`, identityservice.SessionHash("email:developer@example.test"))
	expect(login("developer@example.test", password, nil), 429)
	if err = RunMigrations(ctx, db); err != nil {
		test.Fatal(err)
	}
	assertCount(test, db, 1, `SELECT count(*) FROM password_accounts`)
}

func TestLegacyNullTargetsBackfillIntegration(test *testing.T) {
	newFixture := workspaceTestCluster(test)
	db := newFixture()
	executeFixture(test, db, initSQL)
	executeFixture(test, db, `INSERT INTO access_credentials(id,credential_key,name) VALUES ('11111111-1111-4111-8111-111111111111','test-only','Owner');
	INSERT INTO projects(id,user_id,title,diagram_type,project_mode) VALUES('22222222-2222-4222-8222-222222222222','11111111-1111-4111-8111-111111111111','Legacy project','erd','diagram');
	INSERT INTO chat_messages(project_id,role,content,target_node_ids) VALUES
	('22222222-2222-4222-8222-222222222222','user','JSON null targets','null'::jsonb),
	('22222222-2222-4222-8222-222222222222','assistant','SQL null targets',NULL),
	('22222222-2222-4222-8222-222222222222','user','Legacy object targets','{"node":"orders"}');`)
	if err := RunMigrations(context.Background(), db); err != nil {
		test.Fatal(err)
	}
	assertCount(test, db, 3, `SELECT count(*) FROM chat_messages_v2 WHERE target_element_ids='[]'::jsonb`)
	assertCount(test, db, 1, `SELECT count(*) FROM chat_messages_v2 WHERE metadata->'legacy_target_node_ids'='{"node":"orders"}'::jsonb`)
	if err := RunMigrations(context.Background(), db); err != nil {
		test.Fatal(err)
	}
	assertCount(test, db, 3, `SELECT count(*) FROM chat_messages_v2`)
}

func TestLocalSnapshotPasswordMigrationIntegration(test *testing.T) {
	dump := os.Getenv("PASSWORD_AUTH_MIGRATION_DUMP")
	if dump == "" {
		test.Skip("set PASSWORD_AUTH_MIGRATION_DUMP to test a private local backup on a disposable database")
	}
	newFixture := workspaceTestCluster(test)
	db := newFixture()
	var socket, port, name string
	if err := db.QueryRow(`SELECT current_setting('unix_socket_directories'),current_setting('port'),current_database()`).Scan(&socket, &port, &name); err != nil {
		test.Fatal(err)
	}
	output, err := exec.Command("/usr/lib/postgresql/16/bin/pg_restore", "--no-owner", "--no-privileges", "-h", socket, "-p", port, "-U", "postgres", "-d", name, dump).CombinedOutput()
	if err != nil {
		test.Fatalf("restore disposable snapshot: %v: %s", err, output)
	}
	var before int
	if err = db.QueryRow(`SELECT count(*) FROM projects`).Scan(&before); err != nil {
		test.Fatal(err)
	}
	if err = RunMigrations(context.Background(), db); err != nil {
		test.Fatal(err)
	}
	assertCount(test, db, before, `SELECT count(*) FROM projects`)
	if err = RunMigrations(context.Background(), db); err != nil {
		test.Fatal(err)
	}
	assertCount(test, db, before, `SELECT count(*) FROM projects`)
}
