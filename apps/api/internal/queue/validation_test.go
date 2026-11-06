package queue

import "testing"

// T-011 (unit part): BR-008 v2 range and strict integer grammar.
func TestParsePartySize(t *testing.T) {
	valid := map[string]int{`{"partySize":1}`: 1, `{"partySize":4}`: 4, `{"partySize":10}`: 10, `{"partySize":16}`: 16, ` {"partySize": 8} `: 8}
	for body, want := range valid {
		got, err := ParseJoinRequest([]byte(body))
		if err != nil || got != want {
			t.Errorf("%s: got %d, %v; want %d", body, got, err, want)
		}
	}
	invalid := []string{
		`{"partySize":0}`, `{"partySize":17}`, `{"partySize":-1}`, `{"partySize":1.5}`,
		`{"partySize":4.0}`, `{"partySize":1e1}`, `{"partySize":"4"}`, `{"partySize":null}`,
		`{"partySize":true}`, `{}`, `null`, `[]`, ``, `{"partySize":4} {}`,
		`{"partySize":4,"status":"CALLED"}`, `{"partySize":4,"ticketNumber":1}`,
		`{"partySize":99999999999999999999}`, `{"partySize":04}`,
	}
	for _, body := range invalid {
		if _, err := ParseJoinRequest([]byte(body)); err != ErrValidation {
			t.Errorf("%q accepted or wrong error: %v", body, err)
		}
	}
}

func TestTicketCode(t *testing.T) {
	for _, c := range []struct {
		prefix string
		n      int
		want   string
	}{{"A", 1, "A-001"}, {"A", 14, "A-014"}, {"BX", 999, "BX-999"}, {"A", 1000, "A-1000"}} {
		if got := TicketCode(c.prefix, c.n); got != c.want {
			t.Errorf("TicketCode(%q,%d)=%q want %q", c.prefix, c.n, got, c.want)
		}
	}
}
