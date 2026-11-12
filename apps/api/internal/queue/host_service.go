package queue

import (
	"context"
	"errors"
	"time"

	"dinequeue.local/api/internal/auth"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrQueueEmpty        = errors.New("no waiting entry to call")
	ErrCallInProgress    = errors.New("queue already has a called entry")
	ErrInvalidTransition = errors.New("entry cannot make this status change")
)

type HostQueue struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	RestaurantName string `json:"restaurantName"`
	Timezone       string `json:"timezone"`
	IsActive       bool   `json:"isActive"`
}

// HostEntry is the Host board view: ticket code only, no customer or staff identity.
type HostEntry struct {
	ID          string     `json:"id"`
	QueueID     string     `json:"queueId"`
	TicketCode  string     `json:"ticketCode"`
	ServiceDate string     `json:"serviceDate"`
	PartySize   int        `json:"partySize"`
	Status      string     `json:"status"`
	JoinedAt    time.Time  `json:"joinedAt"`
	CalledAt    *time.Time `json:"calledAt"`
	CompletedAt *time.Time `json:"completedAt"`
}

func utcPointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func hostEntryFromRow(row hostEntryRow) HostEntry {
	return HostEntry{ID: row.ID, QueueID: row.QueueID, TicketCode: TicketCode(row.TicketPrefix, row.SequenceNumber),
		ServiceDate: row.ServiceDate, PartySize: row.PartySize, Status: row.Status, JoinedAt: row.JoinedAt.UTC(),
		CalledAt: utcPointer(row.CalledAt), CompletedAt: utcPointer(row.CompletedAt)}
}

// canonicalID accepts only the 36-character UUID form used in HTTP paths.
func canonicalID(id string) bool {
	if len(id) != 36 {
		return false
	}
	_, err := uuid.Parse(id)
	return err == nil
}

// authorizeHost rechecks role and queue grant on the given repository (a transaction
// handle for mutations), independently of the HTTP middleware.
func authorizeHost(ctx context.Context, repository *Repository, user auth.User, queueID string) error {
	if user.Role != "HOST" {
		return auth.ErrForbidden
	}
	if !canonicalID(queueID) {
		return ErrNotFound
	}
	granted, err := repository.hasGrant(ctx, user.ID, queueID)
	if err != nil {
		return err
	}
	if !granted {
		return auth.ErrForbidden
	}
	return nil
}

func (s *Service) HostQueues(ctx context.Context, user auth.User) ([]HostQueue, error) {
	if user.Role != "HOST" {
		return nil, auth.ErrForbidden
	}
	rows, err := s.repository.assignedQueues(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	queues := make([]HostQueue, 0, len(rows))
	for _, row := range rows {
		queues = append(queues, HostQueue(row))
	}
	return queues, nil
}

// WaitingEntries lists WAITING entries oldest first (BR-003 FIFO).
func (s *Service) WaitingEntries(ctx context.Context, user auth.User, queueID string) ([]HostEntry, error) {
	if err := authorizeHost(ctx, s.repository, user, queueID); err != nil {
		return nil, err
	}
	rows, err := s.repository.entriesByStatus(ctx, queueID, "WAITING")
	if err != nil {
		return nil, err
	}
	entries := make([]HostEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, hostEntryFromRow(row))
	}
	return entries, nil
}

// CurrentEntry returns the queue's CALLED entry, or nil.
func (s *Service) CurrentEntry(ctx context.Context, user auth.User, queueID string) (*HostEntry, error) {
	if err := authorizeHost(ctx, s.repository, user, queueID); err != nil {
		return nil, err
	}
	rows, err := s.repository.entriesByStatus(ctx, queueID, "CALLED")
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	entry := hostEntryFromRow(rows[0])
	return &entry, nil
}

// lockAuthorizedQueue takes the queue lock, then rechecks the grant in the same
// transaction. An unknown queue has no grant, so it is reported as forbidden.
func lockAuthorizedQueue(ctx context.Context, repository *Repository, user auth.User, queueID string) error {
	if user.Role != "HOST" {
		return auth.ErrForbidden
	}
	if !canonicalID(queueID) {
		return ErrNotFound
	}
	found, err := repository.lockQueue(ctx, queueID)
	if err != nil {
		return err
	}
	if !found {
		return auth.ErrForbidden
	}
	return authorizeHost(ctx, repository, user, queueID)
}

// transitionTime keeps lifecycle timestamps ordered even if clocks differ slightly.
func (s *Service) transitionTime(previous time.Time) time.Time {
	at := s.now().UTC().Truncate(time.Microsecond)
	if at.Before(previous) {
		return previous
	}
	return at
}

// CallNext atomically promotes the oldest WAITING entry to CALLED (BR-003, BR-005).
func (s *Service) CallNext(ctx context.Context, user auth.User, queueID string) (HostEntry, error) {
	var entry HostEntry
	err := s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repository := NewRepository(tx)
		if err := lockAuthorizedQueue(ctx, repository, user, queueID); err != nil {
			return err
		}
		called, err := repository.entriesByStatus(ctx, queueID, "CALLED")
		if err != nil {
			return err
		}
		if len(called) > 0 {
			return ErrCallInProgress
		}
		rows, err := repository.oldestWaitingForUpdate(ctx, queueID)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return ErrQueueEmpty
		}
		row := rows[0]
		at := s.transitionTime(row.JoinedAt)
		affected, err := repository.markCalled(ctx, row.ID, user.ID, at)
		if err != nil {
			return err
		}
		if affected != 1 {
			return ErrCallInProgress
		}
		row.Status, row.CalledAt = "CALLED", &at
		entry = hostEntryFromRow(row)
		return nil
	})
	return entry, err
}

// Complete finishes the waitlist service for a CALLED entry (BR-004, legacy F-05).
func (s *Service) Complete(ctx context.Context, user auth.User, queueID, entryID string) (HostEntry, error) {
	// Reject invalid input before opening a transaction, so it never takes the queue lock.
	if user.Role != "HOST" {
		return HostEntry{}, auth.ErrForbidden
	}
	if !canonicalID(queueID) || !canonicalID(entryID) {
		return HostEntry{}, ErrNotFound
	}
	var entry HostEntry
	err := s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repository := NewRepository(tx)
		if err := lockAuthorizedQueue(ctx, repository, user, queueID); err != nil {
			return err
		}
		rows, err := repository.entryForUpdate(ctx, queueID, entryID)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return ErrNotFound
		}
		row := rows[0]
		if row.Status != "CALLED" || row.CalledAt == nil {
			return ErrInvalidTransition
		}
		at := s.transitionTime(*row.CalledAt)
		affected, err := repository.markCompleted(ctx, row.ID, user.ID, at)
		if err != nil {
			return err
		}
		if affected != 1 {
			return ErrInvalidTransition
		}
		row.Status, row.CompletedAt = "COMPLETED", &at
		entry = hostEntryFromRow(row)
		return nil
	})
	return entry, err
}
