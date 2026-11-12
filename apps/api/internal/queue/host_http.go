package queue

import (
	"bytes"
	"errors"

	"dinequeue.local/api/internal/auth"
	"github.com/gofiber/fiber/v3"
)

// registerHost wires F-04 routes. HTTP gates deny early; the service rechecks role
// and queue grant (inside the mutation transaction for Call Next / Complete).
func (h *HTTP) registerHost(router fiber.Router) {
	group := router.Group("/api/v1/host/queues", deadline, h.auth.Authenticate, h.auth.RequireRole("HOST"))
	group.Get("/", h.hostQueues)
	group.Get("/:queueId/entries", h.auth.RequireQueue("queueId"), h.waitingEntries)
	group.Get("/:queueId/current", h.auth.RequireQueue("queueId"), h.currentEntry)
	group.Post("/:queueId/call-next", h.auth.CSRF, h.auth.RequireQueue("queueId"), h.callNext)
	group.Post("/:queueId/entries/:entryId/complete", h.auth.CSRF, h.auth.RequireQueue("queueId"), h.complete)
}

func hostError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, auth.ErrForbidden):
		return failure(c, 403, "FORBIDDEN", "Access denied.")
	case errors.Is(err, ErrNotFound):
		return failure(c, 404, "NOT_FOUND", "Resource not found.")
	case errors.Is(err, ErrQueueEmpty):
		return failure(c, 409, "QUEUE_EMPTY", "No waiting party to call.")
	case errors.Is(err, ErrCallInProgress):
		return failure(c, 409, "CALL_IN_PROGRESS", "Complete the currently called ticket first.")
	case errors.Is(err, ErrInvalidTransition):
		return failure(c, 409, "INVALID_TRANSITION", "Only a called ticket can be completed.")
	}
	return err
}

// Host mutations take no request body; reject one rather than silently ignore it.
func emptyBody(c fiber.Ctx) bool { return len(bytes.TrimSpace(c.Body())) == 0 }

func (h *HTTP) hostQueues(c fiber.Ctx) error {
	user, ok := auth.Identity(c)
	if !ok {
		return failure(c, 401, "UNAUTHENTICATED", "Please sign in.")
	}
	queues, err := h.service.HostQueues(c.Context(), user)
	if err != nil {
		return hostError(c, err)
	}
	return c.JSON(fiber.Map{"queues": queues})
}

func (h *HTTP) waitingEntries(c fiber.Ctx) error {
	if status := c.Query("status"); status != "" && status != "WAITING" {
		return failure(c, 400, "VALIDATION_ERROR", "Only status=WAITING is supported.")
	}
	user, ok := auth.Identity(c)
	if !ok {
		return failure(c, 401, "UNAUTHENTICATED", "Please sign in.")
	}
	entries, err := h.service.WaitingEntries(c.Context(), user, c.Params("queueId"))
	if err != nil {
		return hostError(c, err)
	}
	return c.JSON(fiber.Map{"entries": entries})
}

func (h *HTTP) currentEntry(c fiber.Ctx) error {
	user, ok := auth.Identity(c)
	if !ok {
		return failure(c, 401, "UNAUTHENTICATED", "Please sign in.")
	}
	entry, err := h.service.CurrentEntry(c.Context(), user, c.Params("queueId"))
	if err != nil {
		return hostError(c, err)
	}
	return c.JSON(fiber.Map{"entry": entry})
}

func (h *HTTP) callNext(c fiber.Ctx) error {
	if !emptyBody(c) {
		return failure(c, 400, "VALIDATION_ERROR", "This request does not take a body.")
	}
	user, ok := auth.Identity(c)
	if !ok {
		return failure(c, 401, "UNAUTHENTICATED", "Please sign in.")
	}
	entry, err := h.service.CallNext(c.Context(), user, c.Params("queueId"))
	if err != nil {
		return hostError(c, err)
	}
	return c.JSON(entry)
}

func (h *HTTP) complete(c fiber.Ctx) error {
	if !emptyBody(c) {
		return failure(c, 400, "VALIDATION_ERROR", "This request does not take a body.")
	}
	user, ok := auth.Identity(c)
	if !ok {
		return failure(c, 401, "UNAUTHENTICATED", "Please sign in.")
	}
	entry, err := h.service.Complete(c.Context(), user, c.Params("queueId"), c.Params("entryId"))
	if err != nil {
		return hostError(c, err)
	}
	return c.JSON(entry)
}
