package queue

import (
	"context"
	"strings"
	"time"

	"dinequeue.local/api/internal/auth"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

// NoShowRequest is ActionVersion plus the only Host-selectable reason (F-08, #53).
// expectedVersion is the HELD assignment's version.
type NoShowRequest struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	ReasonCode      string `json:"reasonCode"`
}

// Cancel moves the owner's WAITING ticket to CANCELLED, whatever its readiness and
// even when the queue is closed to new joins. It shares readiness's queue -> entry
// lock order and never takes the floor lock, so it cannot invert seating locks.
// The row (code, join time, number) is kept as history; it is never deleted.
func (s *Service) Cancel(ctx context.Context, user auth.User, entryID string) (Ticket, error) {
	if user.Role != "CUSTOMER" {
		return Ticket{}, auth.ErrForbidden
	}
	if !canonicalID(entryID) {
		return Ticket{}, ErrNotFound
	}
	entryID = strings.ToLower(entryID)
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
		row, err := r.ownedEntryForUpdate(ctx, user.ID, entryID)
		if err != nil {
			return err
		}
		if row.Status != "WAITING" {
			return ErrInvalidTransition
		}
		at := s.transitionTime(row.JoinedAt)
		result := tx.Exec("UPDATE queue_entries SET status='CANCELLED',cancelled_at=?,cancelled_by=?,version=version+1 WHERE id=? AND customer_id=? AND status='WAITING'", at, user.ID, entryID, user.ID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInvalidTransition
		}
		if err := r.event(row.QueueID, entryID, nil, &user.ID, "CUSTOMER_CANCEL", "WAITING", "CANCELLED", map[string]any{}, at); err != nil {
			return err
		}
		row.Status = "CANCELLED"
		ticket = ticketFromRow(row)
		return nil
	})
	return ticket, err
}

// NoShow records CUSTOMER_ABSENT for a CALLED ticket whose assignment is still HELD
// and releases the whole group through the shared releaseHold coordinator. It
// follows Seat's sequence: scope checks, keyed replay, independent expiry cleanup,
// then floor -> queue -> entry -> members locks with every guard rechecked.
func (s *Service) NoShow(ctx context.Context, u auth.User, q, entryID string, request NoShowRequest) (SeatingResult, error) {
	var out SeatingResult
	if err := authorizeHost(ctx, s.repository, u, q); err != nil {
		return out, err
	}
	if !canonicalID(entryID) {
		return out, ErrNotFound
	}
	entryID = strings.ToLower(entryID)
	if !canonicalID(request.IdempotencyKey) || request.ExpectedVersion < 1 || request.ReasonCode != "CUSTOMER_ABSENT" {
		return out, ErrValidation
	}
	request.IdempotencyKey = strings.ToLower(request.IdempotencyKey)
	hash := actionHash("entries/"+entryID+"/no-show", request)
	if found, err := s.prior(ctx, u, q, "no-show", request.IdempotencyKey, hash, &out); err != nil || found {
		return out, err
	}
	// Early rejection only: a foreign or unknown entry must not trigger the
	// independent expiry cleanup below. This unlocked read is NOT the ownership
	// check; entryForUpdate inside the transaction rechecks queue scope under lock.
	var inQueue int64
	if err := s.repository.db.WithContext(ctx).Raw("SELECT count(*) FROM queue_entries WHERE queue_id=? AND id=?", q, entryID).Scan(&inQueue).Error; err != nil {
		return out, err
	}
	if inQueue != 1 {
		return out, ErrNotFound
	}
	if _, err := s.expire(ctx, u, q); err != nil {
		return out, err
	}
	err := s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		r := NewRepository(tx)
		if err := r.lockFloorQueue(ctx, u, q); err != nil {
			return err
		}
		if found, err := r.replay(u, q, "no-show", request.IdempotencyKey, hash, &out); err != nil || found {
			return err
		}
		if err := r.seatingMode(q); err != nil {
			return err
		}
		rows, err := r.entryForUpdate(ctx, q, entryID)
		if err != nil {
			return err
		}
		if len(rows) != 1 {
			return ErrNotFound
		}
		// An entry has at most one assignment: it can be called only from WAITING,
		// and no terminal status returns to WAITING (a rejoin is a new entry). The
		// 0005 partial unique index seating_one_outstanding_entry also enforces at
		// most one HELD/ACTIVE row per entry. ORDER BY/LIMIT only keeps this
		// deterministic; it does not expect several rows.
		var ids []string
		if err := tx.Raw("SELECT id FROM seating_assignments WHERE queue_id=? AND entry_id=? ORDER BY held_at DESC, id DESC LIMIT 1", q, entryID).Scan(&ids).Error; err != nil {
			return err
		}
		if len(ids) != 1 {
			return ErrInvalidTransition
		}
		a, err := r.assignment(q, ids[0])
		if err != nil {
			return err
		}
		if err := r.lockMembers(ctx, a.TableIDs); err != nil {
			return err
		}
		at := s.now().UTC().Truncate(time.Microsecond)
		if a.ReleaseReason != nil && *a.ReleaseReason == "HOLD_EXPIRED" {
			return matchError("HOLD_EXPIRED")
		}
		if a.Status != "HELD" || rows[0].Status != "CALLED" {
			return ErrInvalidTransition
		}
		if !at.Before(a.HoldExpiresAt) {
			return matchError("HOLD_EXPIRED")
		}
		if a.Version != request.ExpectedVersion {
			return matchError("STALE_PREVIEW")
		}
		if rows[0].CalledAt != nil && at.Before(*rows[0].CalledAt) {
			at = rows[0].CalledAt.UTC()
		}
		if err := r.releaseHold(q, a, &u.ID, "CUSTOMER_ABSENT", at); err != nil {
			return err
		}
		out, err = r.seatingResult(ctx, q, a.ID)
		if err != nil {
			return err
		}
		return r.record(u, q, "no-show", request.IdempotencyKey, hash, out, at)
	})
	return out, err
}

func (h *HTTP) cancel(c fiber.Ctx) error {
	user, ok := auth.Identity(c)
	if !ok {
		return failure(c, 401, "UNAUTHENTICATED", "Please sign in.")
	}
	// The contract body is empty; anything else is rejected rather than ignored.
	if !emptyBody(c) {
		return failure(c, 400, "VALIDATION_ERROR", "Cancellation takes no request body.")
	}
	ticket, err := h.service.Cancel(c.Context(), user, c.Params("entryId"))
	if err == ErrInvalidTransition {
		return failure(c, 409, "INVALID_TRANSITION", "Only a waiting ticket can be cancelled.")
	}
	if err != nil {
		return hostError(c, err)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(ticket)
}

func (h *HTTP) noShow(c fiber.Ctx) error {
	var request NoShowRequest
	if err := jsonSeating(c, &request); err != nil {
		return seatingError(c, err)
	}
	user, _ := auth.Identity(c)
	out, err := h.service.NoShow(c.Context(), user, c.Params("queueId"), c.Params("entryId"), request)
	if err == ErrInvalidTransition {
		return failure(c, 409, "INVALID_TRANSITION", "Only a called ticket with a held table group can be marked no-show.")
	}
	if err != nil {
		return seatingError(c, err)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}
