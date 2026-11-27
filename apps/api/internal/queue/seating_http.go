package queue

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"reflect"

	"dinequeue.local/api/internal/auth"
	"github.com/gofiber/fiber/v3"
)

// Reject duplicate keys at every nesting level before decoding typed integers.
func uniqueJSON(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		if t == nil {
			return ErrValidation
		}
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := k.(string)
			if !ok || seen[key] {
				return ErrValidation
			}
			seen[key] = true
			if err := uniqueJSON(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := uniqueJSON(d); err != nil {
				return err
			}
		}
	default:
		return ErrValidation
	}
	_, err = d.Token()
	return err
}
func parseSeating(body []byte, out any) error {
	if len(body) == 0 || len(body) > 4096 || bytes.TrimSpace(body)[0] != '{' {
		return ErrValidation
	}
	d := json.NewDecoder(bytes.NewReader(body))
	if err := uniqueJSON(d); err != nil {
		return ErrValidation
	}
	if _, err := d.Token(); err != io.EOF {
		return ErrValidation
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return ErrValidation
	}
	allowed := map[string]bool{}
	typ := reflect.TypeOf(out).Elem()
	for i := 0; i < typ.NumField(); i++ {
		allowed[typ.Field(i).Tag.Get("json")] = true
	}
	for key, value := range fields {
		if !allowed[key] {
			return ErrValidation
		}
		if key == "expectedTableVersions" {
			var versions []map[string]json.RawMessage
			if json.Unmarshal(value, &versions) != nil {
				return ErrValidation
			}
			for _, v := range versions {
				if len(v) != 2 || v["tableId"] == nil || v["version"] == nil {
					return ErrValidation
				}
			}
		}
	}
	d = json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return ErrValidation
	}
	return nil
}
func seatingError(c fiber.Ctx, err error) error {
	var me matchError
	if errors.Is(err, ErrValidation) {
		return failure(c, 400, "VALIDATION_ERROR", "Invalid seating request.")
	}
	if errors.As(err, &me) {
		status := 409
		if me == "INVALID_TABLE_SET" || me == "VALIDATION_ERROR" {
			status = 400
		}
		return failure(c, status, string(me), "Seating state changed or the selected action is unavailable. Refresh and try again.")
	}
	return hostError(c, err)
}
func jsonSeating(c fiber.Ctx, out any) error {
	media, _, err := mime.ParseMediaType(c.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return ErrValidation
	}
	return parseSeating(c.Body(), out)
}
func (h *HTTP) callHold(c fiber.Ctx) error {
	var request CallHoldRequest
	if err := jsonSeating(c, &request); err != nil {
		return seatingError(c, err)
	}
	user, _ := auth.Identity(c)
	out, err := h.service.CallHold(c.Context(), user, c.Params("queueId"), request)
	if err != nil {
		return seatingError(c, err)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}
func (h *HTTP) seat(c fiber.Ctx) error {
	var request ActionVersion
	if err := jsonSeating(c, &request); err != nil {
		return seatingError(c, err)
	}
	user, _ := auth.Identity(c)
	out, err := h.service.Seat(c.Context(), user, c.Params("queueId"), c.Params("assignmentId"), request)
	if err != nil {
		return seatingError(c, err)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}
func (h *HTTP) expireHolds(c fiber.Ctx) error {
	var request struct {
		IdempotencyKey string `json:"idempotencyKey"`
	}
	if err := jsonSeating(c, &request); err != nil {
		return seatingError(c, err)
	}
	user, _ := auth.Identity(c)
	out, err := h.service.ExpireHolds(c.Context(), user, c.Params("queueId"), request.IdempotencyKey)
	if err != nil {
		return seatingError(c, err)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}
