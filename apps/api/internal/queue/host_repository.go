package queue

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

const oneCalledIndex = "uq_one_called_per_queue"

type hostQueueRow struct {
	ID             string
	Name           string
	RestaurantName string
	Timezone       string
	IsActive       bool
}

type hostEntryRow struct {
	ID             string
	QueueID        string
	ServiceDate    string
	SequenceNumber int
	PartySize      int
	Status         string
	JoinedAt       time.Time
	CalledAt       *time.Time
	CompletedAt    *time.Time
	TicketPrefix   string
}

// Host projection: no customer identity or staff actor IDs are loaded.
const hostProjection = `SELECT e.id, e.queue_id, e.service_date::text AS service_date,
e.sequence_number, e.party_size, e.status, e.joined_at, e.called_at, e.completed_at, q.ticket_prefix
FROM queue_entries AS e JOIN service_queues AS q ON q.id = e.queue_id`

func (r *Repository) assignedQueues(ctx context.Context, hostID string) ([]hostQueueRow, error) {
	var rows []hostQueueRow
	err := r.db.WithContext(ctx).Raw(`SELECT q.id, q.name, r.name AS restaurant_name, r.timezone, q.is_active
FROM operator_queue_permissions AS p JOIN service_queues AS q ON q.id = p.queue_id
JOIN restaurants AS r ON r.id = q.restaurant_id
WHERE p.user_id = ? ORDER BY r.name, q.name, q.id`, hostID).Scan(&rows).Error
	return rows, err
}

func (r *Repository) hasGrant(ctx context.Context, hostID, queueID string) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("operator_queue_permissions").Where("user_id = ? AND queue_id = ?", hostID, queueID).Count(&n).Error
	return n == 1, err
}

// lockQueue serializes Host mutations on one queue. Call Next and Complete both take
// this lock first, so their row locks are always acquired in the same order.
func (r *Repository) lockQueue(ctx context.Context, queueID string) (bool, error) {
	var ids []string
	err := r.db.WithContext(ctx).Raw("SELECT id FROM service_queues WHERE id = ? FOR UPDATE", queueID).Scan(&ids).Error
	return len(ids) == 1, err
}

func (r *Repository) entriesByStatus(ctx context.Context, queueID, status string) ([]hostEntryRow, error) {
	var rows []hostEntryRow
	err := r.db.WithContext(ctx).Raw(hostProjection+`
WHERE e.queue_id = ? AND e.status = ? ORDER BY e.joined_at, e.id`, queueID, status).Scan(&rows).Error
	return rows, err
}

func (r *Repository) oldestWaitingForUpdate(ctx context.Context, queueID string) ([]hostEntryRow, error) {
	var rows []hostEntryRow
	err := r.db.WithContext(ctx).Raw(hostProjection+`
WHERE e.queue_id = ? AND e.status = 'WAITING' ORDER BY e.joined_at, e.id LIMIT 1 FOR UPDATE OF e`, queueID).Scan(&rows).Error
	return rows, err
}

// entryForUpdate is scoped to the path queue so another queue's entry is not found.
func (r *Repository) entryForUpdate(ctx context.Context, queueID, entryID string) ([]hostEntryRow, error) {
	var rows []hostEntryRow
	err := r.db.WithContext(ctx).Raw(hostProjection+`
WHERE e.queue_id = ? AND e.id = ? FOR UPDATE OF e`, queueID, entryID).Scan(&rows).Error
	return rows, err
}

// markCalled is a conditional transition; callers must check one affected row.
func (r *Repository) markCalled(ctx context.Context, entryID, hostID string, at time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Exec(`UPDATE queue_entries SET status = 'CALLED', called_at = ?, called_by = ?, version = version + 1
WHERE id = ? AND status = 'WAITING'`, at, hostID, entryID)
	var pgError *pgconn.PgError
	if errors.As(result.Error, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == oneCalledIndex {
		return 0, ErrCallInProgress
	}
	return result.RowsAffected, result.Error
}

func (r *Repository) markCompleted(ctx context.Context, entryID, hostID string, at time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Exec(`UPDATE queue_entries SET status = 'COMPLETED', completed_at = ?, completed_by = ?, version = version + 1
WHERE id = ? AND status = 'CALLED'`, at, hostID, entryID)
	return result.RowsAffected, result.Error
}
