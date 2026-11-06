package postgres

import (
	"context"

	"gorm.io/gorm"
)

// Readiness queries through the same ORM connection used by future repositories.
type Readiness struct{ database *gorm.DB }

func NewReadiness(database *gorm.DB) *Readiness {
	return &Readiness{database: database}
}

func (r *Readiness) Ping(ctx context.Context) error {
	var value int
	return r.database.WithContext(ctx).Raw("SELECT 1").Scan(&value).Error
}
