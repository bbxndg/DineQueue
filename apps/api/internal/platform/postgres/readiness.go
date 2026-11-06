package postgres

import (
	"context"

	"dinequeue.local/api/internal/db"
)

// Readiness uses a sqlc query through the same pgx pool as future repositories.
type Readiness struct{ queries *db.Queries }

func NewReadiness(connection db.DBTX) *Readiness {
	return &Readiness{queries: db.New(connection)}
}

func (r *Readiness) Ping(ctx context.Context) error {
	_, err := r.queries.CheckDatabase(ctx)
	return err
}
