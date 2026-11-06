package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrCredentials     = errors.New("invalid credentials")
	ErrUnauthenticated = errors.New("authentication required")
	ErrForbidden       = errors.New("access denied")
	ErrNotFound        = errors.New("resource not found")
	ErrBusy            = errors.New("too many attempts")
)

const SessionLifetime = 12 * time.Hour

type Service struct {
	repository *Repository
	now        func() time.Time
	dummyHash  string
	hashSlots  chan struct{}
}

func NewService(repository *Repository) (*Service, error) {
	// Equal-cost password verification for missing accounts without a static credential.
	dummy := make([]byte, 32)
	if _, err := rand.Read(dummy); err != nil {
		return nil, err
	}
	hash, err := HashPassword(base64.RawURLEncoding.EncodeToString(dummy))
	if err != nil {
		return nil, err
	}
	return &Service{repository: repository, now: time.Now, dummyHash: hash, hashSlots: make(chan struct{}, 2)}, nil
}

type LoginResult struct {
	User      User
	Token     string
	ExpiresAt time.Time
}

func (s *Service) Login(ctx context.Context, email, password, oldToken string) (LoginResult, error) {
	select {
	case s.hashSlots <- struct{}{}:
		defer func() { <-s.hashSlots }()
	default:
		return LoginResult{}, ErrBusy
	}
	account, err := s.repository.findCredentials(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil && !errors.Is(err, ErrCredentials) {
		return LoginResult{}, err
	}
	hash := s.dummyHash
	if err == nil {
		hash = account.PasswordHash
	}
	valid := VerifyPassword(hash, password)
	if !valid || err != nil || !account.IsActive {
		return LoginResult{}, ErrCredentials
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return LoginResult{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	now := s.now().UTC()
	expiry := now.Add(SessionLifetime)
	row := sessionRow{ID: uuid.NewString(), UserID: account.ID, TokenHash: TokenHash(token), CreatedAt: now, ExpiresAt: expiry}
	// Service coordinates rotation and creation as one business transaction.
	if err := s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current credentials
		if err := tx.Table("users").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", account.ID).Take(&current).Error; err != nil {
			return err
		}
		if !current.IsActive || current.PasswordHash != account.PasswordHash {
			return ErrCredentials
		}
		account = current
		repository := NewRepository(tx)
		if oldHash := TokenHash(oldToken); oldHash != "" {
			if err := repository.revoke(ctx, oldHash, now); err != nil {
				return err
			}
		}
		return repository.createSession(ctx, row)
	}); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{User: User{ID: account.ID, DisplayName: account.DisplayName, Role: account.Role}, Token: token, ExpiresAt: expiry}, nil
}

func TokenHash(token string) string {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != 32 {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// The opaque session token is a high-entropy, HttpOnly key; the domain-separated
// CSRF value may be exposed to same-origin JS without exposing that cookie.
func CSRFToken(token string) string {
	if TokenHash(token) == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte("dinequeue.csrf.v1"))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func ValidCSRF(token, supplied string) bool {
	expected := CSRFToken(token)
	return expected != "" && len(expected) == len(supplied) && subtle.ConstantTimeCompare([]byte(expected), []byte(supplied)) == 1
}

func (s *Service) Identity(ctx context.Context, token string) (User, error) {
	hash := TokenHash(token)
	if hash == "" {
		return User{}, ErrUnauthenticated
	}
	return s.repository.identity(ctx, hash, s.now().UTC())
}
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.repository.revoke(ctx, TokenHash(token), s.now().UTC())
}
func (s *Service) RequireQueue(ctx context.Context, user User, queueID string) error {
	if user.Role != "HOST" {
		return ErrForbidden
	}
	if _, err := uuid.Parse(queueID); err != nil {
		return ErrNotFound
	}
	allowed, err := s.repository.hasQueuePermission(ctx, user.ID, queueID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

// ownerID must come from a server-loaded resource, never from request input.
func RequireOwner(user User, ownerID string) error {
	if user.Role != "CUSTOMER" || user.ID != ownerID {
		return ErrNotFound
	}
	return nil
}
