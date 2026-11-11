package queue

import (
	"context"
	"errors"
	"mime"
	"time"

	"dinequeue.local/api/internal/auth"
	"github.com/gofiber/fiber/v3"
)

type HTTP struct {
	service *Service
	auth    *auth.HTTP
}

func NewHTTP(service *Service, authHTTP *auth.HTTP) *HTTP {
	return &HTTP{service: service, auth: authHTTP}
}

func (h *HTTP) Register(router fiber.Router) {
	group := router.Group("/api/v1/queues", deadline, h.auth.Authenticate)
	group.Get("/", h.list)
	group.Post("/:queueId/entries", h.auth.CSRF, h.auth.RequireRole("CUSTOMER"), h.join)

	meGroup := router.Group("/api/v1/me", deadline, h.auth.Authenticate, h.auth.RequireRole("CUSTOMER"))
	meGroup.Get("/queue-entries/active", h.listActiveEntries)
	meGroup.Get("/queue-entries/:entryId", h.getEntry)
}

func deadline(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()
	c.SetContext(ctx)
	return c.Next()
}

func failure(c fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": message, "requestId": c.GetRespHeader("X-Request-ID")}})
}

func (h *HTTP) list(c fiber.Ctx) error {
	queues, err := h.service.ActiveQueues(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"queues": queues})
}

func (h *HTTP) join(c fiber.Ctx) error {
	user, ok := auth.Identity(c)
	if !ok {
		return failure(c, 401, "UNAUTHENTICATED", "Please sign in.")
	}
	mediaType, _, err := mime.ParseMediaType(c.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return failure(c, 400, "VALIDATION_ERROR", "Party size must be a whole number from 1 to 16.")
	}
	partySize, err := ParseJoinRequest(c.Body())
	if err != nil {
		return failure(c, 400, "VALIDATION_ERROR", "Party size must be a whole number from 1 to 16.")
	}
	entry, err := h.service.Join(c.Context(), user, c.Params("queueId"), partySize)
	switch {
	case err == nil:
		return c.Status(201).JSON(entry)
	case errors.Is(err, ErrValidation):
		return failure(c, 400, "VALIDATION_ERROR", "Party size must be a whole number from 1 to 16.")
	case errors.Is(err, auth.ErrForbidden):
		return failure(c, 403, "FORBIDDEN", "Access denied.")
	case errors.Is(err, ErrNotFound):
		return failure(c, 404, "NOT_FOUND", "Resource not found.")
	case errors.Is(err, ErrQueueInactive):
		return failure(c, 409, "QUEUE_INACTIVE", "This queue is not accepting new entries.")
	case errors.Is(err, ErrActiveQueueExists):
		return failure(c, 409, "ACTIVE_QUEUE_EXISTS", "You already have an active ticket for this queue.")
	case errors.Is(err, ErrInvalidTimezone):
		return failure(c, 500, "INTERNAL_ERROR", "Restaurant timezone is not configured correctly.")
	}
	return err
}

func (h *HTTP) listActiveEntries(c fiber.Ctx) error {
	user, ok := auth.Identity(c)
	if !ok {
		return failure(c, 401, "UNAUTHENTICATED", "Please sign in.")
	}
	entries, err := h.service.ActiveEntries(c.Context(), user)
	if errors.Is(err, auth.ErrForbidden) {
		return failure(c, 403, "FORBIDDEN", "Access denied.")
	}
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"entries": entries})
}

func (h *HTTP) getEntry(c fiber.Ctx) error {
	user, ok := auth.Identity(c)
	if !ok {
		return failure(c, 401, "UNAUTHENTICATED", "Please sign in.")
	}
	entry, err := h.service.EntryDetail(c.Context(), user, c.Params("entryId"))
	switch {
	case err == nil:
		return c.JSON(entry)
	case errors.Is(err, ErrNotFound):
		return failure(c, 404, "NOT_FOUND", "Resource not found.")
	case errors.Is(err, auth.ErrForbidden):
		return failure(c, 403, "FORBIDDEN", "Access denied.")
	}
	return err
}
