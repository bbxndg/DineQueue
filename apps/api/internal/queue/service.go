package queue

import (
	"context"
	"errors"
	"time"
	_ "time/tzdata" // Restaurant timezones must resolve on hosts without a zoneinfo database.

	"dinequeue.local/api/internal/auth"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrValidation        = errors.New("invalid queue entry request")
	ErrNotFound          = errors.New("resource not found")
	ErrQueueInactive     = errors.New("queue is not accepting entries")
	ErrActiveQueueExists = errors.New("customer already has an active entry")
	ErrInvalidTimezone   = errors.New("restaurant timezone is not configured correctly")
)

type Queue struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	RestaurantName string `json:"restaurantName"`
}

// Entry is the customer-facing summary; it never includes owner identity.
type Entry struct {
	ID          string    `json:"id"`
	QueueID     string    `json:"queueId"`
	TicketCode  string    `json:"ticketCode"`
	ServiceDate string    `json:"serviceDate"`
	PartySize   int       `json:"partySize"`
	Status      string    `json:"status"`
	JoinedAt    time.Time `json:"joinedAt"`
}

// Ticket adds queue context to an owned read without changing the join response.
type Ticket struct {
	Entry
	QueueName      string `json:"queueName"`
	RestaurantName string `json:"restaurantName"`
	Timezone       string `json:"timezone"`
}

func ticketFromRow(row entryDetail) Ticket {
	return Ticket{
		Entry: Entry{ID: row.ID, QueueID: row.QueueID,
			TicketCode:  TicketCode(row.TicketPrefix, row.SequenceNumber),
			ServiceDate: row.ServiceDate, PartySize: row.PartySize,
			Status: row.Status, JoinedAt: row.JoinedAt.UTC()},
		QueueName: row.QueueName, RestaurantName: row.RestaurantName, Timezone: row.Timezone,
	}
}

type Service struct {
	repository *Repository
	now        func() time.Time
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (s *Service) ActiveQueues(ctx context.Context) ([]Queue, error) {
	rows, err := s.repository.activeQueues(ctx)
	if err != nil {
		return nil, err
	}
	queues := make([]Queue, 0, len(rows))
	for _, row := range rows {
		queues = append(queues, Queue{ID: row.ID, Name: row.Name, RestaurantName: row.RestaurantName})
	}
	return queues, nil
}

func (s *Service) ActiveEntries(ctx context.Context, user auth.User) ([]Ticket, error) {
	if user.Role != "CUSTOMER" {
		return nil, auth.ErrForbidden
	}
	rows, err := s.repository.activeEntries(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	entries := make([]Ticket, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, ticketFromRow(row))
	}
	return entries, nil
}

func (s *Service) EntryDetail(ctx context.Context, user auth.User, entryID string) (Ticket, error) {
	if user.Role != "CUSTOMER" {
		return Ticket{}, auth.ErrForbidden
	}
	// uuid.Parse also accepts URNs/raw hex; HTTP IDs use the canonical UUID form.
	if len(entryID) != 36 {
		return Ticket{}, ErrNotFound
	}
	if _, err := uuid.Parse(entryID); err != nil {
		return Ticket{}, ErrNotFound
	}
	row, err := s.repository.ownedEntryByID(ctx, user.ID, entryID)
	if err != nil {
		return Ticket{}, err
	}
	return ticketFromRow(row), nil
}

// Join creates one WAITING entry in a single transaction: queue check, local service
// date, counter allocation and insert commit or roll back together (BR-001/002/009).
func (s *Service) Join(ctx context.Context, user auth.User, queueID string, partySize int) (Entry, error) {
	if user.Role != "CUSTOMER" {
		return Entry{}, auth.ErrForbidden
	}
	if partySize < MinPartySize || partySize > MaxPartySize {
		return Entry{}, ErrValidation
	}
	if _, err := uuid.Parse(queueID); err != nil {
		return Entry{}, ErrNotFound
	}
	var entry Entry
	err := s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repository := NewRepository(tx)
		queue, err := repository.queueForJoin(ctx, queueID)
		if err != nil {
			return err
		}
		if !queue.IsActive {
			return ErrQueueInactive
		}
		location, err := time.LoadLocation(queue.Timezone)
		if err != nil {
			return ErrInvalidTimezone
		}
		joinedAt := s.now().UTC().Truncate(time.Microsecond)
		serviceDate := joinedAt.In(location).Format(time.DateOnly)
		sequence, err := repository.allocateNumber(ctx, queue.ID, serviceDate)
		if err != nil {
			return err
		}
		row := entryRow{ID: uuid.NewString(), QueueID: queue.ID, CustomerID: user.ID, ServiceDate: serviceDate, SequenceNumber: sequence, PartySize: partySize, Status: "WAITING", JoinedAt: joinedAt}
		if err := repository.insertEntry(ctx, row); err != nil {
			return err
		}
		entry = Entry{ID: row.ID, QueueID: row.QueueID, TicketCode: TicketCode(queue.TicketPrefix, sequence), ServiceDate: serviceDate, PartySize: partySize, Status: row.Status, JoinedAt: joinedAt}
		return nil
	})
	if err != nil {
		return Entry{}, err
	}
	return entry, nil
}
