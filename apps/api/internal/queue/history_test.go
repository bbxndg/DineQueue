package queue

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

// Unit coverage for the history query/cursor contract (H06, H05 precision).

func TestHistoryCursorRoundTripKeepsMicroseconds(t *testing.T) {
	at := time.Date(2026, 11, 20, 3, 4, 5, 123456000, time.FixedZone("ICT", 7*3600))
	id := "0a1b2c3d-0000-4000-8000-00000000000f"
	encoded := encodeHistoryCursor(at, id)
	if len(encoded) > 512 || strings.ContainsAny(encoded, "+/=") {
		t.Fatalf("cursor must be unpadded base64url: %q", encoded)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(encoded)
	if !strings.Contains(string(raw), `"2026-11-19T20:04:05.123456Z"`) {
		t.Fatalf("cursor must be UTC with 6 fractional digits: %s", raw)
	}
	c, err := decodeHistoryCursor(encoded)
	if err != nil || !c.JoinedAt.Equal(at) || c.ID != id {
		t.Fatalf("round trip %+v %v", c, err)
	}
	whole := encodeHistoryCursor(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), id)
	raw, _ = base64.RawURLEncoding.DecodeString(whole)
	if !strings.Contains(string(raw), `.000000Z`) {
		t.Fatalf("zero fraction must still have 6 digits: %s", raw)
	}
}

func TestParseHistoryQuery(t *testing.T) {
	good := encodeHistoryCursor(time.Date(2026, 11, 20, 10, 0, 0, 1000, time.UTC), "0a1b2c3d-0000-4000-8000-00000000000f")
	q, err := parseHistoryQuery(nil)
	if err != nil || q.Limit != 20 || q.Cursor != nil {
		t.Fatalf("default %+v %v", q, err)
	}
	q, err = parseHistoryQuery([][2]string{{"limit", "50"}, {"cursor", good}})
	if err != nil || q.Limit != 50 || q.Cursor == nil {
		t.Fatalf("valid %+v %v", q, err)
	}
	for _, bad := range [][][2]string{
		{{"limit", "0"}}, {{"limit", "51"}}, {{"limit", "05"}}, {{"limit", "050"}}, {{"limit", "00"}}, {{"limit", "1.0"}}, {{"limit", " 5"}}, {{"limit", "١"}},
		{{"limit", "5"}, {"limit", "5"}}, {{"cursor", good}, {"cursor", good}}, {{"ownerId", "x"}}, {{"", ""}},
		{{"cursor", ""}}, {{"cursor", strings.Repeat("A", 513)}},
	} {
		if _, err := parseHistoryQuery(bad); err != ErrValidation {
			t.Fatalf("%v accepted: %v", bad, err)
		}
	}
}
