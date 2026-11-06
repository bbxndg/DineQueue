package queue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
)

const (
	MinPartySize = 1
	MaxPartySize = 16 // BR-008 v2; physical table capacity is separately capped at 8.
	maxJoinBody  = 1024
)

// Plain decimal integer without sign, fraction, exponent or leading zero.
var integerLiteral = regexp.MustCompile(`^[1-9][0-9]{0,2}$`)

// ParseJoinRequest accepts exactly {"partySize": <integer 1..16>}. Ticket number,
// status, date and owner always come from the server (BR-009).
func ParseJoinRequest(body []byte) (int, error) {
	if len(body) > maxJoinBody {
		return 0, ErrValidation
	}
	var input struct {
		PartySize json.RawMessage `json:"partySize"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return 0, ErrValidation
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return 0, ErrValidation
	}
	if !integerLiteral.Match(input.PartySize) {
		return 0, ErrValidation
	}
	size, err := strconv.Atoi(string(input.PartySize))
	if err != nil || size < MinPartySize || size > MaxPartySize {
		return 0, ErrValidation
	}
	return size, nil
}

// TicketCode is display-only, e.g. A-014; the UUID remains the resource identifier.
func TicketCode(prefix string, sequence int) string {
	return fmt.Sprintf("%s-%03d", prefix, sequence)
}
