package queue

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"sort"
	"strings"
	"time"

	"dinequeue.local/api/internal/auth"
	"dinequeue.local/api/internal/matching"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type matchError string

func (e matchError) Error() string { return string(e) }

type TableVersion struct {
	TableID string `json:"tableId"`
	Version int64  `json:"version"`
}
type MatchCandidate struct {
	TableIDs        []string       `json:"tableIds"`
	TableVersions   []TableVersion `json:"tableVersions"`
	ZoneID          string         `json:"zoneId"`
	TotalSeats      int            `json:"totalSeats"`
	UnusedSeats     int            `json:"unusedSeats"`
	Rank            int            `json:"rank"`
	SelectionReason string         `json:"selectionReason"`
}
type MatchSkip struct {
	TicketCode string `json:"ticketCode"`
	ReasonCode string `json:"reasonCode"`
}
type MatchPreview struct {
	QueueID            string           `json:"queueId"`
	LayoutVersion      int64            `json:"layoutVersion"`
	ServerNow          time.Time        `json:"serverNow"`
	Entry              *HostEntry       `json:"entry"`
	EntryVersion       *int64           `json:"entryVersion"`
	Candidates         []MatchCandidate `json:"candidates"`
	Skipped            []MatchSkip      `json:"skipped"`
	AvailabilityReason *string          `json:"availabilityReason"`
}

func normalizeSelection(ids []string, entry string) ([]string, string, error) {
	if (len(ids) == 0) == (entry == "") {
		return nil, "", matchError("VALIDATION_ERROR")
	}
	if entry != "" {
		if !canonicalID(entry) {
			return nil, "", ErrNotFound
		}
		return nil, strings.ToLower(entry), nil
	}
	if len(ids) > 3 {
		return nil, "", matchError("INVALID_TABLE_SET")
	}
	seen := map[string]bool{}
	out := []string{}
	for _, id := range ids {
		parsed, err := uuid.Parse(id)
		if err != nil || len(id) != 36 {
			return nil, "", matchError("INVALID_TABLE_SET")
		}
		id = parsed.String()
		if seen[id] {
			return nil, "", matchError("INVALID_TABLE_SET")
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out, "", nil
}
func (s *Service) Matching(ctx context.Context, user auth.User, queueID string, ids []string, entryID string) (MatchPreview, error) {
	p := MatchPreview{Candidates: []MatchCandidate{}, Skipped: []MatchSkip{}}
	if user.Role != "HOST" {
		return p, auth.ErrForbidden
	}
	if !canonicalID(queueID) {
		return p, ErrNotFound
	}
	ids, entryID, err := normalizeSelection(ids, entryID)
	if err != nil {
		return p, err
	}
	err = s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		r := NewRepository(tx)
		if err := authorizeHost(ctx, r, user, queueID); err != nil {
			return err
		}
		floor, err := r.floorSnapshot(ctx, queueID)
		if err != nil {
			return err
		}
		p.QueueID = queueID
		p.LayoutVersion = floor.LayoutVersion
		p.ServerNow = floor.ServerNow
		tables := []matching.Table{}
		edges := [][2]string{}
		byID := map[string]FloorTable{}
		zones := map[string]string{}
		active := map[string]bool{}
		for _, z := range floor.Zones {
			for _, t := range z.Tables {
				byID[t.ID] = t
				zones[t.ID] = z.ID
				active[t.ID] = z.IsActive
				tables = append(tables, matching.Table{ID: t.ID, Zone: z.ID, Capacity: t.Capacity, Available: z.IsActive && t.Status == "AVAILABLE"})
			}
			for _, e := range z.Connections {
				edges = append(edges, e.TableIDs)
			}
		}
		var rows []hostEntryRow
		if err := tx.Raw(strings.Replace(hostProjection, "SELECT e.id,", "SELECT e.version, e.id,", 1)+" WHERE e.queue_id=? AND e.status IN ('WAITING','CALLED') ORDER BY e.joined_at,e.id", queueID).Scan(&rows).Error; err != nil {
			return err
		}
		seats := 0
		if len(ids) > 0 {
			zone := ""
			for _, id := range ids {
				t, ok := byID[id]
				if !ok {
					return ErrNotFound
				}
				if zone != "" && zone != zones[id] {
					return matchError("TABLES_NOT_CONNECTED")
				}
				zone = zones[id]
				if !active[id] {
					return matchError("TABLES_NOT_CONNECTED")
				}
				seats += t.Capacity
			}
			if !matching.Connected(ids, edges) {
				return matchError("TABLES_NOT_CONNECTED")
			}
			for _, id := range ids {
				if byID[id].Status != "AVAILABLE" {
					return matchError("TABLE_NOT_AVAILABLE")
				}
			}
		}
		reason := func(v string) { p.AvailabilityReason = &v }
		if entryID != "" {
			for _, row := range rows {
				if row.ID == entryID {
					if row.Status != "WAITING" || !row.IsReady {
						return matchError("ENTRY_NOT_ELIGIBLE")
					}
					e := hostEntryFromRow(row)
					p.Entry = &e
					v := row.Version
					p.EntryVersion = &v
				}
			}
			if p.Entry == nil {
				var count int64
				if err := tx.Table("queue_entries").Where("id=? AND queue_id=?", entryID, queueID).Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					return matchError("ENTRY_NOT_ELIGIBLE")
				}
				return ErrNotFound
			}
		}
		for _, row := range rows {
			if row.Status == "CALLED" {
				reason("CALL_IN_PROGRESS")
				return nil
			}
		}
		if entryID == "" {
			waiting, ready := 0, 0
			for _, row := range rows {
				waiting++
				why := ""
				if !row.IsReady {
					why = "NOT_READY"
				} else {
					ready++
					if row.PartySize > seats {
						why = "CAPACITY_NOT_ELIGIBLE"
					}
				}
				if why != "" {
					p.Skipped = append(p.Skipped, MatchSkip{TicketCode: TicketCode(row.TicketPrefix, row.SequenceNumber), ReasonCode: why})
					continue
				}
				e := hostEntryFromRow(row)
				p.Entry = &e
				v := row.Version
				p.EntryVersion = &v
				break
			}
			if p.Entry == nil {
				if waiting == 0 {
					reason("QUEUE_EMPTY")
				} else if ready == 0 {
					reason("NO_READY_PARTIES")
				} else {
					reason("NO_FITTING_PARTY")
				}
				return nil
			}
		}
		candidates := matching.Recommend(tables, edges, p.Entry.PartySize)
		for i, c := range candidates {
			if len(ids) > 0 && strings.Join(c.IDs, ",") != strings.Join(ids, ",") {
				continue
			}
			versions := []TableVersion{}
			for _, id := range c.IDs {
				versions = append(versions, TableVersion{TableID: id, Version: byID[id].Version})
			}
			why := "RECOMMENDED"
			if i > 0 {
				why = "ALTERNATIVE_SET"
			}
			p.Candidates = append(p.Candidates, MatchCandidate{TableIDs: c.IDs, TableVersions: versions, ZoneID: c.Zone, TotalSeats: c.Seats, UnusedSeats: c.Seats - p.Entry.PartySize, Rank: i + 1, SelectionReason: why})
		}
		if len(p.Candidates) == 0 {
			reason("NO_MATCH_AVAILABLE")
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return p, err
}
func (h *HTTP) matching(c fiber.Ctx) error {
	query, err := url.ParseQuery(string(c.Request().URI().QueryString()))
	if err != nil {
		return failure(c, 400, "VALIDATION_ERROR", "Invalid matching query.")
	}
	for key := range query {
		if key != "tableId" && key != "entryId" {
			return failure(c, 400, "VALIDATION_ERROR", "Invalid matching query.")
		}
	}
	if len(query["entryId"]) > 1 || (query.Has("entryId") && query.Get("entryId") == "") {
		return failure(c, 400, "VALIDATION_ERROR", "Invalid matching query.")
	}
	user, _ := auth.Identity(c)
	p, err := h.service.Matching(c.Context(), user, c.Params("queueId"), query["tableId"], query.Get("entryId"))
	if err != nil {
		var me matchError
		if errors.As(err, &me) {
			status := 409
			if me == "VALIDATION_ERROR" || me == "INVALID_TABLE_SET" {
				status = 400
			}
			return failure(c, status, string(me), "Matching preview is unavailable for this selection.")
		}
		return hostError(c, err)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(p)
}
