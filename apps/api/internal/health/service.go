package health

import (
	"context"
	"time"
)

// Probe is implemented by the PostgreSQL adapter, and replaced only in unit tests.
type Probe interface{ Ping(context.Context) error }

type Service struct{ probe Probe }

func New(probe Probe) *Service { return &Service{probe: probe} }

func (s *Service) Ready(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.probe.Ping(ctx)
}
