package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/repository"
)

type fakeRepository struct {
	ctx context.Context

	findKeyHash   string
	findKeyCalls  int
	findKeyResult *repository.Credential
	findKeyErr    error

	findIDCalls  int
	findIDResult *repository.Credential
	findIDErr    error

	createCalls  int
	createInput  repository.NewCredential
	createResult *repository.Credential
	createErr    error

	updateCalls  int
	updateInput  repository.UpdateCredential
	updateResult *repository.Credential
	updateErr    error

	deleteCalls int
	deleteErr   error

	listCalls  int
	listResult []repository.CredentialStats
	listErr    error

	lastLoginCalls int
	lastLoginErr   error

	bootstrapCalls int
	bootstrapKey   string
	bootstrapErr   error

	migrateCalls int
	migrateErr   error
}

func (fake *fakeRepository) MigrateCredentialKeys(ctx context.Context) error {
	fake.ctx = ctx
	fake.migrateCalls++
	return fake.migrateErr
}

func (fake *fakeRepository) EnsureMasterAdmin(ctx context.Context, bootstrapKey string) error {
	fake.ctx = ctx
	fake.bootstrapCalls++
	fake.bootstrapKey = bootstrapKey
	return fake.bootstrapErr
}

func (fake *fakeRepository) FindByKeyHash(ctx context.Context, keyHash string) (*repository.Credential, error) {
	fake.ctx = ctx
	fake.findKeyCalls++
	fake.findKeyHash = keyHash
	return fake.findKeyResult, fake.findKeyErr
}

func (fake *fakeRepository) FindByID(ctx context.Context, id string) (*repository.Credential, error) {
	fake.ctx = ctx
	fake.findIDCalls++
	return fake.findIDResult, fake.findIDErr
}

func (fake *fakeRepository) Create(ctx context.Context, credential repository.NewCredential) (*repository.Credential, error) {
	fake.ctx = ctx
	fake.createCalls++
	fake.createInput = credential
	return fake.createResult, fake.createErr
}

func (fake *fakeRepository) Update(ctx context.Context, credential repository.UpdateCredential) (*repository.Credential, error) {
	fake.ctx = ctx
	fake.updateCalls++
	fake.updateInput = credential
	return fake.updateResult, fake.updateErr
}

func (fake *fakeRepository) Delete(ctx context.Context, id string) error {
	fake.ctx = ctx
	fake.deleteCalls++
	return fake.deleteErr
}

func (fake *fakeRepository) UpdateLastLogin(ctx context.Context, id string) error {
	fake.ctx = ctx
	fake.lastLoginCalls++
	return fake.lastLoginErr
}

func (fake *fakeRepository) GetAllWithStats(ctx context.Context) ([]repository.CredentialStats, error) {
	fake.ctx = ctx
	fake.listCalls++
	return fake.listResult, fake.listErr
}

func TestFindByKeyHashesInputMasksResultAndForwardsContext(t *testing.T) {
	type contextKey string
	fake := &fakeRepository{findKeyResult: &repository.Credential{
		ID:        "user-1",
		KeyPrefix: identity.HiddenCredentialPrefix,
		Name:      "Client",
		Role:      "user",
	}}
	ctx := context.WithValue(context.Background(), contextKey("request"), "request-1")
	record, err := NewCredentialService(fake).FindByKey(ctx, "  RL-SECRET  ")
	if err != nil {
		t.Fatalf("unexpected lookup error: %v", err)
	}
	if fake.findKeyHash != identity.HashCredentialKey("RL-SECRET") {
		t.Fatalf("service did not hash normalized token: %q", fake.findKeyHash)
	}
	if record.CredentialKey != "********" || strings.Contains(record.CredentialKey, fake.findKeyHash) {
		t.Fatalf("service returned unmasked credential: %q", record.CredentialKey)
	}
	if fake.ctx.Value(contextKey("request")) != "request-1" {
		t.Fatal("service did not forward request context")
	}
}

func TestFindByKeyRejectsBlankWithoutRepositoryAccess(t *testing.T) {
	fake := &fakeRepository{}
	_, err := NewCredentialService(fake).FindByKey(context.Background(), "  ")
	if err == nil || err.Error() != "credential key is required" {
		t.Fatalf("expected required-key error, got %v", err)
	}
	if fake.findKeyCalls != 0 {
		t.Fatal("blank key must not reach persistence")
	}
}

func TestCreateNormalizesInputStoresDigestAndReturnsRawTokenOnce(t *testing.T) {
	fake := &fakeRepository{createResult: &repository.Credential{
		ID:        "user-1",
		KeyPrefix: "RL-PLACEHOLDER",
		Name:      "Pengguna Baru",
		Role:      "user",
	}}
	record, err := NewCredentialService(fake).Create(context.Background(), CreateInput{
		Name:               "   ",
		Role:               "superuser",
		CanGenerateDiagram: true,
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if fake.createInput.Name != "Pengguna Baru" || fake.createInput.Role != "user" {
		t.Fatalf("service did not normalize input: %+v", fake.createInput)
	}
	if fake.createInput.KeyHash != identity.HashCredentialKey(record.CredentialKey) {
		t.Fatal("persistence must receive digest of returned one-time token")
	}
	if fake.createInput.KeyPrefix != identity.CredentialKeyPrefix(record.CredentialKey) {
		t.Fatal("persistence must receive the token display prefix")
	}
	if len(record.CredentialKey) != 46 || !strings.HasPrefix(record.CredentialKey, "RL-") {
		t.Fatalf("unexpected one-time token: %q", record.CredentialKey)
	}
}

func TestUpdateProtectsMasterAdminFields(t *testing.T) {
	now := time.Now()
	expiry := now.Add(time.Hour)
	fake := &fakeRepository{
		findIDResult: &repository.Credential{
			ID:                 identity.MasterAdminID,
			KeyPrefix:          identity.HiddenCredentialPrefix,
			Name:               "Administrator Utama",
			Role:               "admin",
			IsActive:           true,
			CanGenerateDiagram: true,
			CanGenerateUI:      true,
			ExpiresAt:          &expiry,
			CreatedAt:          now,
			UpdatedAt:          now,
		},
		updateResult: &repository.Credential{
			ID:                 identity.MasterAdminID,
			KeyPrefix:          identity.HiddenCredentialPrefix,
			Name:               "Updated",
			Role:               "admin",
			IsActive:           true,
			CanGenerateDiagram: true,
			CanGenerateUI:      true,
			ExpiresAt:          &expiry,
			CreatedAt:          now,
			UpdatedAt:          now,
		},
	}
	disabled := false
	newExpiry := now.Add(24 * time.Hour)
	record, err := NewCredentialService(fake).Update(context.Background(), UpdateInput{
		ID:                 identity.MasterAdminID,
		Name:               " Updated ",
		Role:               "user",
		IsActive:           &disabled,
		ExpiresAt:          &newExpiry,
		UpdateExpiresAt:    true,
		CanGenerateDiagram: &disabled,
		CanGenerateUI:      &disabled,
	})
	if err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}
	if fake.updateInput.Name != "Updated" {
		t.Fatalf("expected trimmed name, got %q", fake.updateInput.Name)
	}
	if fake.updateInput.Role != "admin" || !fake.updateInput.IsActive || !fake.updateInput.CanGenerateDiagram || !fake.updateInput.CanGenerateUI {
		t.Fatalf("master admin permissions were weakened: %+v", fake.updateInput)
	}
	if fake.updateInput.ExpiresAt == nil || !fake.updateInput.ExpiresAt.Equal(expiry) {
		t.Fatal("master admin expiry must remain unchanged")
	}
	if record.CredentialKey != "********" {
		t.Fatalf("update must return a mask, got %q", record.CredentialKey)
	}
}

func TestDeleteRejectsMasterAdminBeforeRepository(t *testing.T) {
	fake := &fakeRepository{}
	err := NewCredentialService(fake).Delete(context.Background(), identity.MasterAdminID)
	if err == nil || err.Error() != "kredensial Master Admin tidak dapat dihapus" {
		t.Fatalf("expected master admin deletion protection, got %v", err)
	}
	if fake.deleteCalls != 0 {
		t.Fatal("protected delete must not reach persistence")
	}
}

func TestGetAllWithStatsMasksAndFormatsRows(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	expiry := now.Add(48 * time.Hour)
	lastLogin := now.Add(-time.Hour)
	fake := &fakeRepository{listResult: []repository.CredentialStats{{
		Credential: repository.Credential{
			ID:          "user-1",
			KeyPrefix:   "RL-ABCD-",
			Name:        "Client",
			Role:        "user",
			IsActive:    true,
			ExpiresAt:   &expiry,
			LastLoginAt: &lastLogin,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		ProjectCount: 3,
	}}}
	rows, err := NewCredentialService(fake).GetAllWithStats(context.Background())
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("unexpected row count: %d", len(rows))
	}
	if rows[0].CredentialKey != "RL-ABCD-********" || rows[0].ProjectCount != 3 {
		t.Fatalf("unexpected summary mapping: %+v", rows[0])
	}
	if rows[0].ExpiresAt == nil || *rows[0].ExpiresAt != expiry.Format(time.RFC3339) || rows[0].IsExpired {
		t.Fatalf("unexpected expiry mapping: %+v", rows[0])
	}
	if rows[0].LastLoginAt == nil || *rows[0].LastLoginAt != lastLogin.Format(time.RFC3339) {
		t.Fatalf("unexpected last login mapping: %+v", rows[0])
	}
}

func TestServiceReturnsRepositoryErrorsWithoutWrappingAwayIdentity(t *testing.T) {
	failure := errors.New("database unavailable")
	fake := &fakeRepository{findKeyErr: failure}
	_, err := NewCredentialService(fake).FindByKey(context.Background(), "RL-SECRET")
	if !errors.Is(err, failure) {
		t.Fatalf("expected repository failure, got %v", err)
	}
}
