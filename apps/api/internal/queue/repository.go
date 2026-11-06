package queue

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

const activeTicketIndex = "uq_one_active_ticket_per_customer_queue"

type queueRow struct {
	ID             string
	Name           string
	TicketPrefix   string
	IsActive       bool
	RestaurantName string
	Timezone       string
}

type entryRow struct {
	ID             string
	QueueID        string
	CustomerID     string
	ServiceDate    string
	SequenceNumber int
	PartySize      int
	Status         string
	JoinedAt       time.Time
}

type Repository struct{ db *gorm.DB }

func NewRepository(database *gorm.DB) *Repository { return &Repository{db: database} }

func (r *Repository) activeQueues(ctx context.Context) ([]queueRow, error) {
	var rows []queueRow
	err := r.db.WithContext(ctx).Table("service_queues AS q").
		Select("q.id, q.name, q.ticket_prefix, q.is_active, r.name AS restaurant_name, r.timezone").
		Joins("JOIN restaurants AS r ON r.id = q.restaurant_id").
		Where("q.is_active = true").Order("r.name, q.name, q.id").Scan(&rows).Error
	return rows, err
}

// queueForJoin must run on the service transaction: FOR SHARE keeps is_active and the
// ticket prefix stable until commit while letting concurrent joins proceed.
func (r *Repository) queueForJoin(ctx context.Context, queueID string) (queueRow, error) {
	var rows []queueRow
	err := r.db.WithContext(ctx).Raw(`SELECT q.id, q.name, q.ticket_prefix, q.is_active, r.name AS restaurant_name, r.timezone
FROM service_queues AS q JOIN restaurants AS r ON r.id = q.restaurant_id
WHERE q.id = ? FOR SHARE OF q`, queueID).Scan(&rows).Error
	if err != nil {
		return queueRow{}, err
	}
	if len(rows) == 0 {
		return queueRow{}, ErrNotFound
	}
	return rows[0], nil
}

// allocateNumber atomically increments the (queue, local day) counter. The row lock is
// held until the surrounding transaction ends, so a rollback also returns the number.
func (r *Repository) allocateNumber(ctx context.Context, queueID, serviceDate string) (int, error) {
	var next int
	err := r.db.WithContext(ctx).Raw(`INSERT INTO queue_day_counters (queue_id, service_date, last_number) VALUES (?, ?, 1)
ON CONFLICT (queue_id, service_date) DO UPDATE SET last_number = queue_day_counters.last_number + 1
RETURNING last_number`, queueID, serviceDate).Scan(&next).Error
	return next, err
}

type entryDetail struct {
	entryRow
	TicketPrefix string
}

// activeEntries returns all WAITING/CALLED entries for the given customer across queues.
func (r *Repository) activeEntries(ctx context.Context, customerID string) ([]entryDetail, error) {
	var rows []entryDetail
	err := r.db.WithContext(ctx).Raw(`SELECT e.id, e.queue_id, e.customer_id, e.service_date, e.sequence_number, e.party_size, e.status, e.joined_at, q.ticket_prefix
FROM queue_entries AS e JOIN service_queues AS q ON q.id = e.queue_id
WHERE e.customer_id = ? AND e.status IN ('WAITING','CALLED')
ORDER BY e.joined_at`, customerID).Scan(&rows).Error
	return rows, err
}

// entryByID returns entry with its queue ticket_prefix; returns ErrNotFound if no row matches.
func (r *Repository) entryByID(ctx context.Context, entryID string) (entryDetail, error) {
	var rows []entryDetail
	err := r.db.WithContext(ctx).Raw(`SELECT e.id, e.queue_id, e.customer_id, e.service_date, e.sequence_number, e.party_size, e.status, e.joined_at, q.ticket_prefix
FROM queue_entries AS e JOIN service_queues AS q ON q.id = e.queue_id
WHERE e.id = ?`, entryID).Scan(&rows).Error
	if err != nil {
		return entryDetail{}, err
	}
	if len(rows) == 0 {
		return entryDetail{}, ErrNotFound
	}
	return rows[0], nil
}

func (r *Repository) insertEntry(ctx context.Context, row entryRow) error {
	err := r.db.WithContext(ctx).Exec(`INSERT INTO queue_entries (id, queue_id, customer_id, service_date, sequence_number, party_size, status, joined_at)
VALUES (?, ?, ?, ?, ?, ?, 'WAITING', ?)`, row.ID, row.QueueID, row.CustomerID, row.ServiceDate, row.SequenceNumber, row.PartySize, row.JoinedAt).Error
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == activeTicketIndex {
		return ErrActiveQueueExists
	}
	return err
}
