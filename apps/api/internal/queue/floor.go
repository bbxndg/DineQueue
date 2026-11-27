package queue

import (
	"context"
	"database/sql"
	"time"

	"dinequeue.local/api/internal/auth"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

type FloorTable struct {
	ID        string  `json:"id"`
	Code      string  `json:"code"`
	Capacity  int     `json:"capacity"`
	Status    string  `json:"status"`
	PositionX float64 `json:"positionX"`
	PositionY float64 `json:"positionY"`
	Version   int64   `json:"version"`
}
type FloorConnection struct {
	TableIDs [2]string `json:"tableIds"`
}
type FloorZone struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	IsActive    bool              `json:"isActive"`
	Tables      []FloorTable      `json:"tables" gorm:"-"`
	Connections []FloorConnection `json:"connections" gorm:"-"`
}
type FloorSnapshot struct {
	QueueID        string      `json:"queueId"`
	RestaurantID   string      `json:"restaurantId"`
	OperationsMode string      `json:"operationsMode"`
	LayoutVersion  int64       `json:"layoutVersion"`
	ServerNow      time.Time   `json:"serverNow"`
	HoldTTLSeconds int         `json:"holdTtlSeconds"`
	Zones          []FloorZone `json:"zones" gorm:"-"`
	// Allocation schema belongs to #52. Inventory alone has no assignments.
	Assignments []struct{} `json:"assignments" gorm:"-"`
}

// Floor uses a read-only repeatable-read transaction, including its grant check.
// Polling never creates floor rows, expires holds or changes any business state.
func (s *Service) Floor(ctx context.Context, user auth.User, queueID string) (FloorSnapshot, error) {
	var snapshot FloorSnapshot
	if user.Role != "HOST" {
		return snapshot, auth.ErrForbidden
	}
	if !canonicalID(queueID) {
		return snapshot, ErrNotFound
	}
	err := s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repository := NewRepository(tx)
		if err := authorizeHost(ctx, repository, user, queueID); err != nil {
			return err
		}
		var err error
		snapshot, err = repository.floorSnapshot(ctx, queueID)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return snapshot, err
}

func (h *HTTP) floor(c fiber.Ctx) error {
	user, ok := auth.Identity(c)
	if !ok {
		return failure(c, 401, "UNAUTHENTICATED", "Please sign in.")
	}
	snapshot, err := h.service.Floor(c.Context(), user, c.Params("queueId"))
	if err != nil {
		return hostError(c, err)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(snapshot)
}
