package queue

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"dinequeue.local/api/internal/auth"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

// turnoverRule describes one whole-group turnover transition (F-09/F-16, #54).
// Every member table moves together; claims are retained through DIRTY and
// CLEANING and released only when cleaning finishes.
type turnoverRule struct {
	action             string // event action and request-key operation
	status, phase      string // required assignment status/turnover phase ("" = NULL)
	tableFrom, tableTo string
	eventFrom, eventTo string
}

var turnoverRules = map[string]turnoverRule{
	"checkout":        {"CHECKOUT", "ACTIVE", "", "OCCUPIED", "DIRTY", "ACTIVE", "DIRTY"},
	"start-cleaning":  {"START_CLEANING", "CLOSED", "DIRTY", "DIRTY", "CLEANING", "DIRTY", "CLEANING"},
	"finish-cleaning": {"FINISH_CLEANING", "CLOSED", "CLEANING", "CLEANING", "AVAILABLE", "CLEANING", "CLEANED"},
}

func (s *Service) Checkout(ctx context.Context, u auth.User, q, id string, request ActionVersion) (SeatingResult, error) {
	return s.turnover(ctx, u, q, id, "checkout", request)
}
func (s *Service) StartCleaning(ctx context.Context, u auth.User, q, id string, request ActionVersion) (SeatingResult, error) {
	return s.turnover(ctx, u, q, id, "start-cleaning", request)
}
func (s *Service) FinishCleaning(ctx context.Context, u auth.User, q, id string, request ActionVersion) (SeatingResult, error) {
	return s.turnover(ctx, u, q, id, "finish-cleaning", request)
}

// turnover follows Seat's sequence: scope checks, keyed replay, independent
// expiry housekeeping, then floor -> queue -> entry -> sorted members, with
// every guard rechecked after locking.
func (s *Service) turnover(ctx context.Context, u auth.User, q, id, name string, request ActionVersion) (SeatingResult, error) {
	var out SeatingResult
	rule := turnoverRules[name]
	if err := authorizeHost(ctx, s.repository, u, q); err != nil {
		return out, err
	}
	if !canonicalID(id) {
		return out, ErrNotFound
	}
	id = strings.ToLower(id)
	if !canonicalID(request.IdempotencyKey) || request.ExpectedVersion < 1 {
		return out, ErrValidation
	}
	request.IdempotencyKey = strings.ToLower(request.IdempotencyKey)
	hash := actionHash("seating-assignments/"+id+"/"+name, request)
	if found, err := s.prior(ctx, u, q, name, request.IdempotencyKey, hash, &out); err != nil || found {
		return out, err
	}
	// Reject foreign targets before independent housekeeping.
	if _, err := s.repository.assignment(q, id); err != nil {
		return out, err
	}
	if _, err := s.expire(ctx, u, q); err != nil {
		return out, err
	}
	err := s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		r := NewRepository(tx)
		if err := r.lockFloorQueue(ctx, u, q); err != nil {
			return err
		}
		if found, err := r.replay(u, q, name, request.IdempotencyKey, hash, &out); err != nil || found {
			return err
		}
		if err := r.seatingMode(q); err != nil {
			return err
		}
		a, err := r.assignment(q, id)
		if err != nil {
			return err
		}
		if rows, err := r.entryForUpdate(ctx, q, a.EntryID); err != nil {
			return err
		} else if len(rows) != 1 {
			return ErrNotFound
		}
		if err := r.lockMembers(ctx, a.TableIDs); err != nil {
			return err
		}
		phase := ""
		if a.TurnoverPhase != nil {
			phase = *a.TurnoverPhase
		}
		// A CLEANED (historical) or RELEASED group never matches a rule, so it can
		// never touch tables that were allocated again later.
		if a.Status != rule.status || phase != rule.phase {
			return ErrInvalidTransition
		}
		if a.Version != request.ExpectedVersion {
			return matchError("STALE_PREVIEW")
		}
		// A DIRTY/CLEANING group must still own every member claim (claims are only
		// released by finish-cleaning or no-show/expiry). A mismatch means corrupted
		// data, so fail as an internal error rather than telling the Host to retry.
		var claims int64
		if err := tx.Raw("SELECT count(*) FROM seating_assignment_tables WHERE assignment_id=? AND released_at IS NULL", id).Scan(&claims).Error; err != nil {
			return err
		}
		if claims != int64(len(a.TableIDs)) {
			return errTurnoverClaimInvariant
		}
		at := s.now().UTC().Truncate(time.Microsecond)
		for _, previous := range []*time.Time{a.SeatedAt, a.ClosedAt, a.CleaningStartedAt} {
			if previous != nil && at.Before(*previous) {
				at = previous.UTC()
			}
		}
		if err := checked(tx.Exec("UPDATE dining_tables SET status=?,version=version+1,updated_at=? WHERE id IN ? AND status=?", rule.tableTo, at, a.TableIDs, rule.tableFrom), int64(len(a.TableIDs))); err != nil {
			return err
		}
		var update *gorm.DB
		switch name {
		case "checkout":
			update = tx.Exec("UPDATE seating_assignments SET status='CLOSED',turnover_phase='DIRTY',closed_at=?,version=version+1 WHERE id=? AND status='ACTIVE' AND turnover_phase IS NULL AND version=?", at, id, request.ExpectedVersion)
		case "start-cleaning":
			update = tx.Exec("UPDATE seating_assignments SET turnover_phase='CLEANING',cleaning_started_at=?,version=version+1 WHERE id=? AND status='CLOSED' AND turnover_phase='DIRTY' AND version=?", at, id, request.ExpectedVersion)
		default:
			if err := checked(tx.Exec("UPDATE seating_assignment_tables SET released_at=? WHERE assignment_id=? AND released_at IS NULL", at, id), int64(len(a.TableIDs))); err != nil {
				return err
			}
			update = tx.Exec("UPDATE seating_assignments SET turnover_phase='CLEANED',cleaned_at=?,released_at=?,release_reason='CLEANED',version=version+1 WHERE id=? AND status='CLOSED' AND turnover_phase='CLEANING' AND version=?", at, at, id, request.ExpectedVersion)
		}
		if err := checked(update, 1); err != nil {
			return err
		}
		if err := r.event(q, a.EntryID, &id, &u.ID, rule.action, rule.eventFrom, rule.eventTo, map[string]any{"tableIds": a.TableIDs}, at); err != nil {
			return err
		}
		out, err = r.seatingResult(ctx, q, id)
		if err != nil {
			return err
		}
		return r.record(u, q, name, request.IdempotencyKey, hash, out, at)
	})
	return out, err
}

func (h *HTTP) turnoverHandler(name string) fiber.Handler {
	return func(c fiber.Ctx) error {
		var request ActionVersion
		if err := jsonSeating(c, &request); err != nil {
			return seatingError(c, err)
		}
		user, _ := auth.Identity(c)
		out, err := h.service.turnover(c.Context(), user, c.Params("queueId"), c.Params("assignmentId"), name, request)
		if err == ErrInvalidTransition {
			return failure(c, 409, "INVALID_TRANSITION", "This table group is not in the required state for that step. Refresh the floor.")
		}
		if err != nil {
			return seatingError(c, err)
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(out)
	}
}

var (
	ErrActivationFloor        = errors.New("seating activation requires a configured floor with an active zone and tables")
	ErrActivationLegacyCalled = errors.New("seating activation refused: complete the legacy CALLED ticket first")
	ErrActivationConfirmation = errors.New("seating activation requires the exact current database name")
	// ErrActivationProductionConsent: APP_ENV=production needs --i-understand-production.
	ErrActivationProductionConsent = errors.New("seating activation in production requires --i-understand-production")
	// errTurnoverClaimInvariant: a DIRTY/CLEANING group lost a member claim. That is
	// corrupted data, not a client state error, so it surfaces as a 500.
	errTurnoverClaimInvariant = errors.New("queue: turnover group no longer owns every member claim")
	activationDatabaseName    = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
)

// EnableSeating is the explicit, guarded activation CLI operation (S2-T-044). It
// never runs on API startup. It requires the exact current database name, a
// configured floor and no legacy CALLED ticket, and changes only the queue's
// operations_mode under floor -> queue locks. WAITING tickets, readiness,
// history, counters and credentials are untouched; no assignment is fabricated.
// Re-running on an already SEATING queue is a no-op. In production it also needs
// explicit operator consent, and it reports the queue and mode before/after so the
// operator sees exactly what changed (no secrets).
func EnableSeating(ctx context.Context, database *gorm.DB, queueID, confirmedDatabase string, options ActivationOptions) (ActivationResult, error) {
	out := ActivationResult{QueueID: strings.ToLower(queueID), Database: confirmedDatabase}
	if !canonicalID(queueID) || !activationDatabaseName.MatchString(confirmedDatabase) {
		return out, ErrActivationConfirmation
	}
	if options.Environment == "production" && !options.ConfirmProduction {
		return out, ErrActivationProductionConsent
	}
	queueID = out.QueueID
	err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var actual string
		if err := tx.Raw("SELECT current_database()").Scan(&actual).Error; err != nil {
			return err
		}
		if actual != confirmedDatabase {
			return ErrActivationConfirmation
		}
		var restaurants []string
		if err := tx.Raw("SELECT restaurant_id FROM service_queues WHERE id=?", queueID).Scan(&restaurants).Error; err != nil {
			return err
		}
		if len(restaurants) != 1 {
			return ErrNotFound
		}
		var floors []string
		if err := tx.Raw("SELECT restaurant_id FROM restaurant_floor_state WHERE restaurant_id=? FOR UPDATE", restaurants[0]).Scan(&floors).Error; err != nil {
			return err
		}
		if len(floors) != 1 {
			return ErrActivationFloor
		}
		var locked struct {
			RestaurantID   string
			OperationsMode string
			Name           string
		}
		if err := tx.Raw("SELECT restaurant_id,operations_mode,name FROM service_queues WHERE id=? FOR UPDATE", queueID).Scan(&locked).Error; err != nil {
			return err
		}
		if locked.RestaurantID != restaurants[0] {
			return ErrActivationFloor
		}
		out.QueueName, out.ModeBefore, out.ModeAfter = locked.Name, locked.OperationsMode, locked.OperationsMode
		if locked.OperationsMode == "SEATING" {
			return nil
		}
		var usable int64
		if err := tx.Raw("SELECT count(*) FROM dining_tables t JOIN floor_zones z ON z.id=t.zone_id WHERE t.restaurant_id=? AND z.is_active", restaurants[0]).Scan(&usable).Error; err != nil {
			return err
		}
		if usable == 0 {
			return ErrActivationFloor
		}
		var called int64
		if err := tx.Raw("SELECT count(*) FROM queue_entries WHERE queue_id=? AND status='CALLED'", queueID).Scan(&called).Error; err != nil {
			return err
		}
		if called != 0 {
			return ErrActivationLegacyCalled
		}
		if err := checked(tx.Exec("UPDATE service_queues SET operations_mode='SEATING' WHERE id=? AND operations_mode='MANUAL'", queueID), 1); err != nil {
			return err
		}
		out.ModeAfter = "SEATING"
		return nil
	})
	if err != nil {
		out.ModeAfter = out.ModeBefore
	}
	return out, err
}

// ActivationOptions carries the operator's environment and explicit consent.
type ActivationOptions struct {
	Environment       string
	ConfirmProduction bool
}

// ActivationResult reports what activation changed; it contains no secrets.
type ActivationResult struct {
	QueueID, QueueName, Database string
	ModeBefore, ModeAfter        string
}
