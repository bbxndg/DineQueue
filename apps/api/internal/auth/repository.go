package auth

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

type credentials struct {
	ID           string
	DisplayName  string
	Role         string
	Email        string
	PasswordHash string
	IsActive     bool
}

type sessionRow struct {
	ID        string
	UserID    string
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type Repository struct{ db *gorm.DB }

func NewRepository(database *gorm.DB) *Repository { return &Repository{db: database} }

func (r *Repository) findCredentials(ctx context.Context, email string) (credentials, error) {
	var rows []credentials
	err := r.db.WithContext(ctx).Table("users").Where("email = ?", email).Limit(1).Find(&rows).Error
	if err != nil {
		return credentials{}, err
	}
	if len(rows) == 0 {
		return credentials{}, ErrCredentials
	}
	return rows[0], nil
}

func (r *Repository) createSession(ctx context.Context, row sessionRow) error {
	return r.db.WithContext(ctx).Table("sessions").Create(&row).Error
}

// r must be bound to the service-owned login transaction so the lock is held
// through credential revalidation, session rotation and commit/rollback.
func (r *Repository) credentialsForUpdate(ctx context.Context, userID string) (credentials, error) {
	var current credentials
	err := r.db.WithContext(ctx).Table("users").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).Take(&current).Error
	return current, err
}

func (r *Repository) identity(ctx context.Context, hash string, now time.Time) (User, error) {
	var users []User
	err := r.db.WithContext(ctx).Table("sessions AS s").Select("u.id, u.display_name, u.role").Joins("JOIN users AS u ON u.id = s.user_id").Where("s.token_hash = ? AND s.revoked_at IS NULL AND s.expires_at > ? AND u.is_active = true", hash, now).Limit(1).Scan(&users).Error
	if err != nil {
		return User{}, err
	}
	if len(users) == 0 {
		return User{}, ErrUnauthenticated
	}
	return users[0], nil
}

func (r *Repository) revoke(ctx context.Context, hash string, now time.Time) error {
	return r.db.WithContext(ctx).Table("sessions").Where("token_hash = ? AND revoked_at IS NULL", hash).Update("revoked_at", now).Error
}

func (r *Repository) hasQueuePermission(ctx context.Context, userID, queueID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("operator_queue_permissions").Where("user_id = ? AND queue_id = ?", userID, queueID).Count(&count).Error
	return count > 0, err
}
