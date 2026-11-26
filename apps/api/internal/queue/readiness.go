package queue

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"

	"dinequeue.local/api/internal/auth"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

// ParseReadinessRequest requires exactly one boolean, including rejecting duplicate
// keys and case aliases rather than letting encoding/json silently choose a value.
func ParseReadinessRequest(body []byte) (bool, error) {
	if len(body) > maxJoinBody {
		return false, ErrValidation
	}
	d := json.NewDecoder(bytes.NewReader(body))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return false, ErrValidation
	}
	if !d.More() {
		return false, ErrValidation
	}
	key, err := d.Token()
	if err != nil || key != "isReady" {
		return false, ErrValidation
	}
	token, err = d.Token()
	ready, ok := token.(bool)
	if err != nil || !ok {
		return false, ErrValidation
	}
	if d.More() {
		return false, ErrValidation
	}
	token, err = d.Token()
	if err != nil || token != json.Delim('}') {
		return false, ErrValidation
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return false, ErrValidation
	}
	return ready, nil
}

// SetReadiness shares queue -> entry locking with CallNext/Complete. The initial
// owner-scoped lookup finds the queue only; every guard is repeated under locks.
func (s *Service) SetReadiness(ctx context.Context, user auth.User, entryID string, ready bool) (Ticket, error) {
	if user.Role != "CUSTOMER" {
		return Ticket{}, auth.ErrForbidden
	}
	if !canonicalID(entryID) {
		return Ticket{}, ErrNotFound
	}
	located, err := s.repository.ownedEntryByID(ctx, user.ID, entryID)
	if err != nil {
		return Ticket{}, err
	}
	var ticket Ticket
	err = s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		r := NewRepository(tx)
		found, err := r.lockQueue(ctx, located.QueueID)
		if err != nil {
			return err
		}
		if !found {
			return ErrNotFound
		}
		// Ownership and lifecycle are rechecked on the locked row.
		row, err := r.ownedEntryForUpdate(ctx, user.ID, entryID)
		if err != nil {
			return err
		}
		if row.Status != "WAITING" {
			return ErrInvalidTransition
		}
		if row.IsReady != ready {
			affected, err := r.setReadiness(ctx, user.ID, entryID, ready)
			if err != nil {
				return err
			}
			if affected != 1 {
				return ErrInvalidTransition
			}
			row.IsReady = ready
		}
		ticket = ticketFromRow(row)
		return nil
	})
	return ticket, err
}

func (h *HTTP) setReadiness(c fiber.Ctx) error {
	user, ok := auth.Identity(c)
	if !ok {
		return failure(c, 401, "UNAUTHENTICATED", "Please sign in.")
	}
	media, _, err := mime.ParseMediaType(c.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return failure(c, 400, "VALIDATION_ERROR", "Readiness must be a boolean.")
	}
	ready, err := ParseReadinessRequest(c.Body())
	if err != nil {
		return failure(c, 400, "VALIDATION_ERROR", "Readiness must be a boolean.")
	}
	ticket, err := h.service.SetReadiness(c.Context(), user, c.Params("entryId"), ready)
	if err == ErrInvalidTransition {
		return failure(c, 409, "INVALID_TRANSITION", "Readiness can change only while waiting.")
	}
	if err != nil {
		return hostError(c, err)
	}
	return c.JSON(ticket)
}
