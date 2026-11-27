package queue

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"dinequeue.local/api/internal/auth"
	"github.com/gofiber/fiber/v3"
)

// Customer Ticket History (Sprint 3 Feat 2, issue #56; S3-AC-01..03).
//
// Ownership always comes from the session. The repository combines owner +
// terminal status + exclusive keyset boundary on every page; a cursor only
// moves the boundary and can never name or widen the owner.

const (
	historyDefaultLimit = 20
	historyMaxLimit     = 50
	historyMaxCursor    = 512
	// Fixed 6 fractional digits: PostgreSQL timestamptz precision is microseconds.
	historyCursorLayout = "2006-01-02T15:04:05.000000Z"
)

type historyCursor struct {
	JoinedAt time.Time
	ID       string
}

type historyQuery struct {
	Limit  int
	Cursor *historyCursor
}

// HistoryPage is the GET /me/queue-entries/history response.
type HistoryPage struct {
	Tickets    []Ticket `json:"tickets"`
	NextCursor *string  `json:"nextCursor"`
}

func encodeHistoryCursor(at time.Time, id string) string {
	payload, _ := json.Marshal(struct {
		V        int    `json:"v"`
		JoinedAt string `json:"joinedAt"`
		ID       string `json:"id"`
	}{1, at.UTC().Format(historyCursorLayout), id})
	return base64.RawURLEncoding.EncodeToString(payload)
}

// decodeHistoryCursor accepts only the exact server-generated envelope.
func decodeHistoryCursor(value string) (historyCursor, error) {
	if value == "" || len(value) > historyMaxCursor {
		return historyCursor{}, ErrValidation
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil {
		return historyCursor{}, ErrValidation
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if err := uniqueJSON(d); err != nil {
		return historyCursor{}, ErrValidation
	}
	if _, err := d.Token(); err != io.EOF {
		return historyCursor{}, ErrValidation
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || len(fields) != 3 {
		return historyCursor{}, ErrValidation
	}
	var version int
	var joined, id string
	if string(fields["v"]) != "1" || json.Unmarshal(fields["v"], &version) != nil ||
		json.Unmarshal(fields["joinedAt"], &joined) != nil || json.Unmarshal(fields["id"], &id) != nil {
		return historyCursor{}, ErrValidation
	}
	at, err := time.Parse(historyCursorLayout, joined)
	if err != nil || at.UTC().Format(historyCursorLayout) != joined {
		return historyCursor{}, ErrValidation
	}
	if !canonicalID(id) || strings.ToLower(id) != id {
		return historyCursor{}, ErrValidation
	}
	return historyCursor{JoinedAt: at, ID: id}, nil
}

// parseHistoryQuery accepts at most one limit and one cursor and nothing else
// (owner selectors and unknown keys are rejected rather than ignored).
func parseHistoryQuery(pairs [][2]string) (historyQuery, error) {
	q := historyQuery{Limit: historyDefaultLimit}
	seen := map[string]bool{}
	for _, pair := range pairs {
		key, value := pair[0], pair[1]
		if seen[key] {
			return q, ErrValidation
		}
		seen[key] = true
		switch key {
		case "limit":
			if value == "" || len(value) > 2 || strings.Trim(value, "0123456789") != "" {
				return q, ErrValidation
			}
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > historyMaxLimit {
				return q, ErrValidation
			}
			q.Limit = n
		case "cursor":
			c, err := decodeHistoryCursor(value)
			if err != nil {
				return q, ErrValidation
			}
			q.Cursor = &c
		default:
			return q, ErrValidation
		}
	}
	return q, nil
}

// History returns one page of the CUSTOMER's own terminal tickets. Each page is
// an independent read; no transaction spans requests.
func (s *Service) History(ctx context.Context, user auth.User, q historyQuery) (HistoryPage, error) {
	page := HistoryPage{Tickets: []Ticket{}}
	if user.Role != "CUSTOMER" {
		return page, auth.ErrForbidden
	}
	rows, err := s.repository.historyEntries(ctx, user.ID, q.Cursor, q.Limit+1)
	if err != nil {
		return page, err
	}
	more := len(rows) > q.Limit
	if more {
		rows = rows[:q.Limit]
	}
	for _, row := range rows {
		page.Tickets = append(page.Tickets, ticketFromRow(row))
	}
	if more {
		last := rows[len(rows)-1]
		next := encodeHistoryCursor(last.JoinedAt, last.ID)
		page.NextCursor = &next
	}
	return page, nil
}

// historyEntries: owner + terminal predicate + exclusive (joined_at,id) boundary,
// newest first. Parameterized; the boundary uses the cursor's full microseconds.
func (r *Repository) historyEntries(ctx context.Context, customerID string, cursor *historyCursor, limit int) ([]entryDetail, error) {
	var rows []entryDetail
	where := `
WHERE e.customer_id = ? AND e.status IN ('COMPLETED','CANCELLED','NO_SHOW')`
	args := []any{customerID}
	if cursor != nil {
		where += ` AND (e.joined_at, e.id) < (?, ?::uuid)`
		args = append(args, cursor.JoinedAt, cursor.ID)
	}
	args = append(args, limit)
	err := r.db.WithContext(ctx).Raw(ticketProjection+where+`
ORDER BY e.joined_at DESC, e.id DESC
LIMIT ?`, args...).Scan(&rows).Error
	return rows, err
}

func (h *HTTP) listHistory(c fiber.Ctx) error {
	user, ok := auth.Identity(c)
	if !ok {
		return failure(c, 401, "UNAUTHENTICATED", "Please sign in.")
	}
	c.Set("Cache-Control", "no-store")
	var pairs [][2]string
	for key, value := range c.Request().URI().QueryArgs().All() {
		pairs = append(pairs, [2]string{string(key), string(value)})
	}
	q, err := parseHistoryQuery(pairs)
	if err != nil {
		return failure(c, 400, "VALIDATION_ERROR", "Invalid history query.")
	}
	page, err := h.service.History(c.Context(), user, q)
	if err == auth.ErrForbidden {
		return failure(c, 403, "FORBIDDEN", "Access denied.")
	}
	if err != nil {
		return err
	}
	return c.JSON(page)
}
