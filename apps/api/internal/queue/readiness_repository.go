package queue

import "context"

// Only the entry row is locked; the service already holds its queue lock.
func (r *Repository) ownedEntryForUpdate(ctx context.Context, customerID, entryID string) (entryDetail, error) {
	var rows []entryDetail
	err := r.db.WithContext(ctx).Raw(ticketProjection+`
WHERE e.customer_id = ? AND e.id = ? FOR UPDATE OF e`, customerID, entryID).Scan(&rows).Error
	if err != nil {
		return entryDetail{}, err
	}
	if len(rows) != 1 {
		return entryDetail{}, ErrNotFound
	}
	return rows[0], nil
}

func (r *Repository) setReadiness(ctx context.Context, customerID, entryID string, ready bool) (int64, error) {
	result := r.db.WithContext(ctx).Exec(`UPDATE queue_entries SET is_ready = ?, version = version + 1
WHERE id = ? AND customer_id = ? AND status = 'WAITING'`, ready, entryID, customerID)
	return result.RowsAffected, result.Error
}
