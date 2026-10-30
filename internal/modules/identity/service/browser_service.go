package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/repository"
)

var ErrLogin = errors.New("email atau kata sandi salah")
var ErrLoginLimit = errors.New("terlalu banyak percobaan login; coba lagi nanti")

type BrowserUserFinder interface {
	FindByID(context.Context, string) (*CredentialRecord, error)
}

type BrowserService struct {
	repo          repository.BrowserRepository
	users         BrowserUserFinder
	dummyHash     string
	passwordSlots chan struct{}
}

func NewBrowserService(repo repository.BrowserRepository, users BrowserUserFinder) *BrowserService {
	dummy, err := identity.HashPassword("dummy-password-not-an-account")
	if err != nil {
		panic("cannot initialize password verifier")
	}
	return &BrowserService{repo: repo, users: users, dummyHash: dummy, passwordSlots: make(chan struct{}, 4)}
}

func SessionHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func CSRFToken(token string) string {
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte("visual-workspace-browser-csrf-v1"))
	return hex.EncodeToString(mac.Sum(nil))
}

func (service *BrowserService) Login(ctx context.Context, email, password, ip, previous string) (string, *CredentialRecord, error) {
	canonical, err := identity.NormalizeEmail(email)
	if err != nil || len(password) == 0 || len(password) > 256 {
		return "", nil, ErrLogin
	}
	allowed, err := service.repo.ClaimLogin(ctx, SessionHash("email:"+canonical), SessionHash("ip:"+ip))
	if err != nil {
		return "", nil, err
	}
	if !allowed {
		return "", nil, ErrLoginLimit
	}
	select {
	case service.passwordSlots <- struct{}{}:
		defer func() { <-service.passwordSlots }()
	default:
		return "", nil, ErrLoginLimit
	}
	account, err := service.repo.AccountByEmail(ctx, canonical)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", nil, err
	}
	encoded := service.dummyHash
	if err == nil {
		encoded = account.PasswordHash
	}
	valid := identity.CheckPassword(encoded, password)
	if !valid || err != nil {
		return "", nil, ErrLogin
	}
	user, err := service.users.FindByID(ctx, account.UserID)
	if err != nil || !activeBrowserUser(user) {
		return "", nil, ErrLogin
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	if err = service.repo.CreateSession(ctx, SessionHash(token), user.ID, SessionHash(previous), time.Now().Add(12*time.Hour)); err != nil {
		return "", nil, err
	}
	return token, user, nil
}

func (service *BrowserService) Authenticate(ctx context.Context, token string) (*CredentialRecord, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 32 {
		return nil, ErrLogin
	}
	userID, err := service.repo.SessionUser(ctx, SessionHash(token))
	if err != nil {
		return nil, ErrLogin
	}
	user, err := service.users.FindByID(ctx, userID)
	if err != nil || !activeBrowserUser(user) {
		return nil, ErrLogin
	}
	return user, nil
}

func activeBrowserUser(user *CredentialRecord) bool {
	return user != nil && user.IsActive && (user.ExpiresAt == nil || time.Now().Before(*user.ExpiresAt))
}

func (service *BrowserService) Email(ctx context.Context, userID string) (string, error) {
	account, err := service.repo.AccountByID(ctx, userID)
	if err != nil {
		return "", err
	}
	return account.Email, nil
}

func (service *BrowserService) Logout(ctx context.Context, token string) error {
	return service.repo.DeleteSession(ctx, SessionHash(token))
}
