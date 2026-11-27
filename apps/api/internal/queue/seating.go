package queue

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"dinequeue.local/api/internal/auth"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CallHoldRequest struct {
	EntryID               string         `json:"entryId"`
	TableIDs              []string       `json:"tableIds"`
	ExpectedLayoutVersion int64          `json:"expectedLayoutVersion"`
	ExpectedEntryVersion  int64          `json:"expectedEntryVersion"`
	ExpectedTableVersions []TableVersion `json:"expectedTableVersions"`
	IdempotencyKey        string         `json:"idempotencyKey"`
	SelectionReason       string         `json:"selectionReason"`
}
type ActionVersion struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
}
type Assignment struct {
	ID                string     `json:"id"`
	QueueID           string     `json:"queueId"`
	EntryID           string     `json:"entryId"`
	ZoneID            string     `json:"zoneId"`
	Status            string     `json:"status"`
	TurnoverPhase     *string    `json:"turnoverPhase"`
	Version           int64      `json:"version"`
	TableIDs          []string   `json:"tableIds" gorm:"-"`
	HeldAt            time.Time  `json:"heldAt"`
	HoldExpiresAt     time.Time  `json:"holdExpiresAt"`
	SeatedAt          *time.Time `json:"seatedAt"`
	ClosedAt          *time.Time `json:"closedAt"`
	CleaningStartedAt *time.Time `json:"cleaningStartedAt"`
	CleanedAt         *time.Time `json:"cleanedAt"`
	ReleasedAt        *time.Time `json:"releasedAt"`
	ReleaseReason     *string    `json:"releaseReason"`
}
type SeatingResult struct {
	Entry      HostEntry    `json:"entry"`
	Assignment Assignment   `json:"assignment"`
	Tables     []FloorTable `json:"tables"`
}
type ExpiryResult struct {
	ExpiredAssignments []string  `json:"expiredAssignments"`
	ServerNow          time.Time `json:"serverNow"`
}

func normalizeCall(r CallHoldRequest) (CallHoldRequest, error) {
	r.ExpectedTableVersions = append([]TableVersion(nil), r.ExpectedTableVersions...)
	ids, _, err := normalizeSelection(r.TableIDs, "")
	if err != nil {
		return r, err
	}
	r.TableIDs = ids
	if !canonicalID(r.EntryID) || !canonicalID(r.IdempotencyKey) || r.ExpectedLayoutVersion < 1 || r.ExpectedEntryVersion < 1 {
		return r, ErrValidation
	}
	r.EntryID = strings.ToLower(r.EntryID)
	r.IdempotencyKey = strings.ToLower(r.IdempotencyKey)
	if r.SelectionReason != "RECOMMENDED" && r.SelectionReason != "ALTERNATIVE_SET" {
		return r, ErrValidation
	}
	if len(r.ExpectedTableVersions) != len(ids) {
		return r, matchError("INVALID_TABLE_SET")
	}
	for i := range r.ExpectedTableVersions {
		v := &r.ExpectedTableVersions[i]
		if !canonicalID(v.TableID) || v.Version < 1 {
			return r, ErrValidation
		}
		v.TableID = strings.ToLower(v.TableID)
	}
	sort.Slice(r.ExpectedTableVersions, func(i, j int) bool { return r.ExpectedTableVersions[i].TableID < r.ExpectedTableVersions[j].TableID })
	for i, id := range ids {
		if r.ExpectedTableVersions[i].TableID != id {
			return r, matchError("INVALID_TABLE_SET")
		}
	}
	return r, nil
}
func actionHash(operation string, payload any) string {
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(append([]byte("POST:"+operation+":"), data...))
	return hex.EncodeToString(sum[:])
}

func (r *Repository) lockFloorQueue(ctx context.Context, u auth.User, q string) error {
	if err := authorizeHost(ctx, r, u, q); err != nil {
		return err
	}
	var restaurant string
	if err := r.db.Raw("SELECT restaurant_id FROM service_queues WHERE id=?", q).Scan(&restaurant).Error; err != nil {
		return err
	}
	var floors []string
	if err := r.db.Raw("SELECT restaurant_id FROM restaurant_floor_state WHERE restaurant_id=? FOR UPDATE", restaurant).Scan(&floors).Error; err != nil {
		return err
	}
	if len(floors) != 1 {
		return matchError("SEATING_NOT_ENABLED")
	}
	if err := lockAuthorizedQueue(ctx, r, u, q); err != nil {
		return err
	}
	var current string
	if err := r.db.Raw("SELECT restaurant_id FROM service_queues WHERE id=?", q).Scan(&current).Error; err != nil {
		return err
	}
	if current != restaurant {
		return matchError("STALE_PREVIEW")
	}
	return nil
}
func (r *Repository) replay(u auth.User, q, op, key, hash string, out any) (bool, error) {
	var rows []struct {
		RequestHash string
		Response    []byte
	}
	if err := r.db.Raw("SELECT request_hash,response FROM operation_requests WHERE actor_id=? AND queue_id=? AND operation=? AND idempotency_key=?", u.ID, q, op, key).Scan(&rows).Error; err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return false, nil
	}
	if rows[0].RequestHash != hash {
		return false, matchError("IDEMPOTENCY_CONFLICT")
	}
	return true, json.Unmarshal(rows[0].Response, out)
}
func (r *Repository) record(u auth.User, q, op, key, hash string, out any, at time.Time) error {
	data, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return r.db.Exec("INSERT INTO operation_requests(actor_id,queue_id,operation,idempotency_key,request_hash,response,created_at) VALUES(?,?,?,?,?,?::jsonb,?)", u.ID, q, op, key, hash, string(data), at).Error
}
func (r *Repository) seatingMode(q string) error {
	var mode string
	if err := r.db.Raw("SELECT operations_mode FROM service_queues WHERE id=?", q).Scan(&mode).Error; err != nil {
		return err
	}
	if mode != "SEATING" {
		return matchError("SEATING_NOT_ENABLED")
	}
	return nil
}
func (r *Repository) assignment(q, id string) (Assignment, error) {
	var rows []Assignment
	if err := r.db.Raw("SELECT * FROM seating_assignments WHERE queue_id=? AND id=?", q, id).Scan(&rows).Error; err != nil {
		return Assignment{}, err
	}
	if len(rows) != 1 {
		return Assignment{}, ErrNotFound
	}
	a := rows[0]
	a.TableIDs = []string{}
	err := r.db.Raw("SELECT table_id FROM seating_assignment_tables WHERE assignment_id=? ORDER BY table_id", id).Scan(&a.TableIDs).Error
	return a, err
}
func (r *Repository) lockMembers(ctx context.Context, ids []string) error {
	var locked []string
	if err := r.db.WithContext(ctx).Raw("SELECT id FROM dining_tables WHERE id IN ? ORDER BY id FOR UPDATE", ids).Scan(&locked).Error; err != nil {
		return err
	}
	if len(locked) != len(ids) {
		return ErrNotFound
	}
	return nil
}
func checked(result *gorm.DB, want int64) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != want {
		return matchError("STALE_PREVIEW")
	}
	return nil
}
func (r *Repository) seatingResult(ctx context.Context, q, id string) (SeatingResult, error) {
	a, err := r.assignment(q, id)
	if err != nil {
		return SeatingResult{}, err
	}
	rows, err := r.entriesByStatus(ctx, q, map[string]string{"HELD": "CALLED", "ACTIVE": "COMPLETED", "RELEASED": "NO_SHOW"}[a.Status])
	if err != nil {
		return SeatingResult{}, err
	}
	out := SeatingResult{Assignment: a, Tables: []FloorTable{}}
	for _, row := range rows {
		if row.ID == a.EntryID {
			out.Entry = hostEntryFromRow(row)
		}
	}
	if out.Entry.ID == "" {
		return out, ErrNotFound
	}
	err = r.db.Raw("SELECT id,code,capacity,status,position_x,position_y,version FROM dining_tables WHERE id IN ? ORDER BY id", a.TableIDs).Scan(&out.Tables).Error
	return out, err
}
func (r *Repository) event(q, entry, assignment string, actor *string, action, from, to string, metadata any, at time.Time) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return r.db.Exec("INSERT INTO operation_events(id,queue_id,entry_id,assignment_id,actor_id,action,from_state,to_state,metadata,occurred_at) VALUES(?,?,?,?,?,?,?,?,?::jsonb,?)", uuid.NewString(), q, entry, assignment, actor, action, from, to, string(data), at).Error
}

// Expiry is an independent committed operation. Callers recheck successful replay
// before invoking it; GET previews and polling never invoke this coordinator.
func (s *Service) expire(ctx context.Context, u auth.User, q string) (ExpiryResult, error) {
	out := ExpiryResult{ExpiredAssignments: []string{}}
	err := s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		r := NewRepository(tx)
		if err := r.lockFloorQueue(ctx, u, q); err != nil {
			return err
		}
		if err := r.seatingMode(q); err != nil {
			return err
		}
		var ids []string
		if err := tx.Raw("SELECT id FROM seating_assignments WHERE queue_id=? AND status='HELD' ORDER BY id", q).Scan(&ids).Error; err != nil {
			return err
		}
		for _, id := range ids {
			a, err := r.assignment(q, id)
			if err != nil {
				return err
			}
			if _, err := r.entryForUpdate(ctx, q, a.EntryID); err != nil {
				return err
			}
			if err := r.lockMembers(ctx, a.TableIDs); err != nil {
				return err
			}
			at := s.now().UTC().Truncate(time.Microsecond)
			out.ServerNow = at
			if at.Before(a.HoldExpiresAt) {
				continue
			}
			if err := checked(tx.Exec("UPDATE queue_entries SET status='NO_SHOW',no_show_at=?,no_show_reason='HOLD_EXPIRED',version=version+1 WHERE id=? AND status='CALLED'", at, a.EntryID), 1); err != nil {
				return err
			}
			if err := checked(tx.Exec("UPDATE dining_tables SET status='AVAILABLE',version=version+1,updated_at=? WHERE id IN ? AND status='HELD'", at, a.TableIDs), int64(len(a.TableIDs))); err != nil {
				return err
			}
			if err := checked(tx.Exec("UPDATE seating_assignment_tables SET released_at=? WHERE assignment_id=? AND released_at IS NULL", at, id), int64(len(a.TableIDs))); err != nil {
				return err
			}
			if err := checked(tx.Exec("UPDATE seating_assignments SET status='RELEASED',released_at=?,release_reason='HOLD_EXPIRED',version=version+1 WHERE id=? AND status='HELD'", at, id), 1); err != nil {
				return err
			}
			if err := r.event(q, a.EntryID, id, nil, "EXPIRE_HOLD", "HELD", "RELEASED", map[string]any{"tableIds": a.TableIDs, "reasonCode": "HOLD_EXPIRED"}, at); err != nil {
				return err
			}
			out.ExpiredAssignments = append(out.ExpiredAssignments, id)
		}
		if out.ServerNow.IsZero() {
			out.ServerNow = s.now().UTC()
		}
		return nil
	})
	return out, err
}
func (s *Service) prior(ctx context.Context, u auth.User, q, op, key, hash string, out any) (bool, error) {
	if err := authorizeHost(ctx, s.repository, u, q); err != nil {
		return false, err
	}
	return s.repository.replay(u, q, op, key, hash, out)
}
func (s *Service) CallHold(ctx context.Context, u auth.User, q string, request CallHoldRequest) (SeatingResult, error) {
	var out SeatingResult
	if err := authorizeHost(ctx, s.repository, u, q); err != nil {
		return out, err
	}
	request, err := normalizeCall(request)
	if err != nil {
		return out, err
	}
	op := "match-and-call"
	hash := actionHash(op, request)
	if found, err := s.prior(ctx, u, q, op, request.IdempotencyKey, hash, &out); err != nil || found {
		return out, err
	}
	if _, err := s.expire(ctx, u, q); err != nil {
		return out, err
	}
	err = s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		r := NewRepository(tx)
		if err := r.lockFloorQueue(ctx, u, q); err != nil {
			return err
		}
		if found, err := r.replay(u, q, op, request.IdempotencyKey, hash, &out); err != nil || found {
			return err
		}
		if err := r.seatingMode(q); err != nil {
			return err
		}
		// Lock the expected entry before member tables; queue lock stabilizes readiness/FIFO.
		if rows, err := r.entryForUpdate(ctx, q, request.EntryID); err != nil {
			return err
		} else if len(rows) != 1 {
			return ErrNotFound
		}
		called, err := r.entriesByStatus(ctx, q, "CALLED")
		if err != nil {
			return err
		}
		if len(called) > 0 {
			return ErrCallInProgress
		}
		var scoped int64
		if err := tx.Raw("SELECT count(*) FROM dining_tables t JOIN service_queues q ON q.restaurant_id=t.restaurant_id WHERE q.id=? AND t.id IN ?", q, request.TableIDs).Scan(&scoped).Error; err != nil {
			return err
		}
		if scoped != int64(len(request.TableIDs)) {
			return ErrNotFound
		}
		if err := r.lockMembers(ctx, request.TableIDs); err != nil {
			return err
		}
		previewService := NewService(r)
		p, err := previewService.Matching(ctx, u, q, request.TableIDs, "")
		if err != nil {
			return err
		}
		if p.AvailabilityReason != nil && *p.AvailabilityReason == "CALL_IN_PROGRESS" {
			return ErrCallInProgress
		}
		if p.LayoutVersion != request.ExpectedLayoutVersion {
			return matchError("STALE_PREVIEW")
		}
		var entryVersion int64
		if err := tx.Raw("SELECT version FROM queue_entries WHERE id=?", request.EntryID).Scan(&entryVersion).Error; err != nil {
			return err
		}
		if entryVersion != request.ExpectedEntryVersion {
			return matchError("STALE_PREVIEW")
		}
		var requested struct {
			PartySize int
			Status    string
			IsReady   bool
		}
		if err := tx.Raw("SELECT party_size,status,is_ready FROM queue_entries WHERE id=? AND queue_id=?", request.EntryID, q).Scan(&requested).Error; err != nil {
			return err
		}
		if requested.Status != "WAITING" || !requested.IsReady {
			return matchError("ENTRY_NOT_ELIGIBLE")
		}
		var selectedSeats int
		if err := tx.Raw("SELECT sum(capacity) FROM dining_tables WHERE id IN ?", request.TableIDs).Scan(&selectedSeats).Error; err != nil {
			return err
		}
		if selectedSeats < requested.PartySize {
			return matchError("CAPACITY_INSUFFICIENT")
		}
		if p.Entry == nil {
			return matchError("ENTRY_NOT_ELIGIBLE")
		}
		if p.Entry.ID != request.EntryID {
			return matchError("QUEUE_ORDER_CHANGED")
		}
		if len(p.Candidates) != 1 {
			return matchError("CAPACITY_INSUFFICIENT")
		}
		candidate := p.Candidates[0]
		for i, v := range candidate.TableVersions {
			if v != request.ExpectedTableVersions[i] {
				return matchError("STALE_PREVIEW")
			}
		}
		if request.SelectionReason != candidate.SelectionReason {
			return ErrValidation
		}
		at := s.transitionTime(p.Entry.JoinedAt)
		id := uuid.NewString()
		var restaurant string
		var ttl int
		if err := tx.Raw("SELECT restaurant_id,hold_ttl_seconds FROM service_queues WHERE id=?", q).Row().Scan(&restaurant, &ttl); err != nil {
			return err
		}
		if err := tx.Exec("INSERT INTO seating_assignments(id,queue_id,entry_id,restaurant_id,zone_id,status,capacity_snapshot,held_at,hold_expires_at) VALUES(?,?,?,?,?,'HELD',?,?,?)", id, q, request.EntryID, restaurant, candidate.ZoneID, candidate.TotalSeats, at, at.Add(time.Duration(ttl)*time.Second)).Error; err != nil {
			return err
		}
		for _, table := range candidate.TableVersions {
			if err := tx.Exec("INSERT INTO seating_assignment_tables(assignment_id,table_id,restaurant_id,zone_id,capacity_snapshot) SELECT ?,id,restaurant_id,zone_id,capacity FROM dining_tables WHERE id=?", id, table.TableID).Error; err != nil {
				return err
			}
			if err := checked(tx.Exec("UPDATE dining_tables SET status='HELD',version=version+1,updated_at=? WHERE id=? AND status='AVAILABLE' AND version=?", at, table.TableID, table.Version), 1); err != nil {
				return err
			}
		}
		n, err := r.markCalled(ctx, request.EntryID, u.ID, at)
		if err != nil {
			return err
		}
		if n != 1 {
			return matchError("QUEUE_ORDER_CHANGED")
		}
		if err := r.event(q, request.EntryID, id, &u.ID, "CALL_HOLD", "WAITING", "HELD", map[string]any{"tableIds": request.TableIDs, "selectionReason": request.SelectionReason, "skipped": p.Skipped}, at); err != nil {
			return err
		}
		out, err = r.seatingResult(ctx, q, id)
		if err != nil {
			return err
		}
		return r.record(u, q, op, request.IdempotencyKey, hash, out, at)
	})
	return out, err
}
func (s *Service) Seat(ctx context.Context, u auth.User, q, id string, request ActionVersion) (SeatingResult, error) {
	var out SeatingResult
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
	op := "seating-assignments/" + id + "/seat"
	hash := actionHash(op, request)
	if found, err := s.prior(ctx, u, q, "seat", request.IdempotencyKey, hash, &out); err != nil || found {
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
		if found, err := r.replay(u, q, "seat", request.IdempotencyKey, hash, &out); err != nil || found {
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
		at := s.now().UTC().Truncate(time.Microsecond)
		if a.ReleaseReason != nil && *a.ReleaseReason == "HOLD_EXPIRED" {
			return matchError("HOLD_EXPIRED")
		}
		if a.Status != "HELD" {
			return ErrInvalidTransition
		}
		if !at.Before(a.HoldExpiresAt) {
			return matchError("HOLD_EXPIRED")
		}
		if a.Version != request.ExpectedVersion {
			return matchError("STALE_PREVIEW")
		}
		if err := checked(tx.Exec("UPDATE dining_tables SET status='OCCUPIED',version=version+1,updated_at=? WHERE id IN ? AND status='HELD'", at, a.TableIDs), int64(len(a.TableIDs))); err != nil {
			return err
		}
		if err := checked(tx.Exec("UPDATE queue_entries SET status='COMPLETED',completed_at=?,completed_by=?,version=version+1 WHERE id=? AND status='CALLED'", at, u.ID, a.EntryID), 1); err != nil {
			return err
		}
		if err := checked(tx.Exec("UPDATE seating_assignments SET status='ACTIVE',seated_at=?,version=version+1 WHERE id=? AND status='HELD' AND version=?", at, id, request.ExpectedVersion), 1); err != nil {
			return err
		}
		if err := r.event(q, a.EntryID, id, &u.ID, "SEAT", "HELD", "ACTIVE", map[string]any{"tableIds": a.TableIDs}, at); err != nil {
			return err
		}
		out, err = r.seatingResult(ctx, q, id)
		if err != nil {
			return err
		}
		return r.record(u, q, "seat", request.IdempotencyKey, hash, out, at)
	})
	return out, err
}
func (s *Service) ExpireHolds(ctx context.Context, u auth.User, q, key string) (ExpiryResult, error) {
	var out ExpiryResult
	if !canonicalID(key) {
		return out, ErrValidation
	}
	key = strings.ToLower(key)
	if err := authorizeHost(ctx, s.repository, u, q); err != nil {
		return out, err
	}
	// Explicit cleanup is idempotent in state; record its result under the same floor lock.
	hash := actionHash("expire-holds", key)
	if found, err := s.prior(ctx, u, q, "expire-holds", key, hash, &out); err != nil || found {
		return out, err
	}
	err := s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		r := NewRepository(tx)
		if err := r.lockFloorQueue(ctx, u, q); err != nil {
			return err
		}
		if found, err := r.replay(u, q, "expire-holds", key, hash, &out); err != nil || found {
			return err
		}
		nested := NewService(r)
		nested.now = s.now
		var err error
		out, err = nested.expire(ctx, u, q)
		if err != nil {
			return err
		}
		return r.record(u, q, "expire-holds", key, hash, out, out.ServerNow)
	})
	return out, err
}
