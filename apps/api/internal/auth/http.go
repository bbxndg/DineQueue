package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

type HTTP struct {
	service    *Service
	origin     string
	secure     bool
	CookieName string
	limiter    *loginLimiter
}

func NewHTTP(service *Service, origin string) *HTTP {
	secure := strings.HasPrefix(origin, "https://")
	name := "dineq_session"
	if secure {
		name = "__Host-dineq_session"
	}
	return &HTTP{service: service, origin: strings.TrimSuffix(origin, "/"), secure: secure, CookieName: name, limiter: &loginLimiter{entries: make(map[string]attempts), now: time.Now}}
}

func (h *HTTP) Register(router fiber.Router) {
	group := router.Group("/api/v1/auth", h.deadline)
	group.Post("/login", h.login)
	group.Get("/me", h.Authenticate, h.me)
	group.Post("/logout", h.Authenticate, h.CSRF, h.logout)
}

func (h *HTTP) deadline(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()
	c.SetContext(ctx)
	return c.Next()
}

func failure(c fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": message, "requestId": c.GetRespHeader("X-Request-ID")}})
}

func (h *HTTP) login(c fiber.Ctx) error {
	if c.Get("Origin") != h.origin {
		return failure(c, 403, "FORBIDDEN", "Request origin is not allowed.")
	}
	if !h.limiter.allow(c.IP()) {
		c.Set("Retry-After", "60")
		return failure(c, 429, "RATE_LIMITED", "Please wait before trying again.")
	}
	mediaType, _, err := mime.ParseMediaType(c.Get("Content-Type"))
	if err != nil || mediaType != "application/json" || len(c.Body()) > 4096 {
		return failure(c, 400, "VALIDATION_ERROR", "Provide a valid email and password.")
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&input)
	var extra any
	if err != nil || decoder.Decode(&extra) != io.EOF {
		return failure(c, 400, "VALIDATION_ERROR", "Provide a valid email and password.")
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	address, err := mail.ParseAddress(input.Email)
	if err != nil || address.Address != input.Email || len(input.Email) > 254 || len(input.Password) == 0 || len(input.Password) > 128 {
		return failure(c, 400, "VALIDATION_ERROR", "Provide a valid email and password.")
	}
	result, err := h.service.Login(c.Context(), input.Email, input.Password, c.Cookies(h.CookieName))
	if errors.Is(err, ErrCredentials) {
		log.Print("authentication rejected: invalid credentials")
		return failure(c, 401, "INVALID_CREDENTIALS", "Email or password is incorrect.")
	}
	if errors.Is(err, ErrBusy) {
		c.Set("Retry-After", "1")
		return failure(c, 429, "RATE_LIMITED", "Please wait before trying again.")
	}
	if err != nil {
		return err
	}
	h.setCookie(c, result.Token, result.ExpiresAt, int(SessionLifetime.Seconds()))
	return c.JSON(fiber.Map{"user": result.User, "csrfToken": CSRFToken(result.Token)})
}

func (h *HTTP) setCookie(c fiber.Ctx, value string, expiry time.Time, maxAge int) {
	c.Cookie(&fiber.Cookie{Name: h.CookieName, Value: value, Path: "/", HTTPOnly: true, Secure: h.secure, SameSite: "Lax", Expires: expiry, MaxAge: maxAge})
}

func (h *HTTP) Authenticate(c fiber.Ctx) error {
	user, err := h.service.Identity(c.Context(), c.Cookies(h.CookieName))
	if errors.Is(err, ErrUnauthenticated) {
		log.Print("session rejected: unauthenticated")
		h.setCookie(c, "", time.Unix(1, 0), -1)
		return failure(c, 401, "UNAUTHENTICATED", "Please sign in.")
	}
	if err != nil {
		return err
	}
	c.Locals("auth.user", user)
	return c.Next()
}

func Identity(c fiber.Ctx) (User, bool) { user, ok := c.Locals("auth.user").(User); return user, ok }

func (h *HTTP) RequireRole(role string) fiber.Handler {
	return func(c fiber.Ctx) error {
		user, ok := Identity(c)
		if !ok {
			return failure(c, 401, "UNAUTHENTICATED", "Please sign in.")
		}
		if user.Role != role {
			return failure(c, 403, "FORBIDDEN", "Access denied.")
		}
		return c.Next()
	}
}

// This HTTP gate is an early denial. Future mutation services must recheck scoped
// permission on their own transaction before writing queue resources.
func (h *HTTP) RequireQueue(param string) fiber.Handler {
	return func(c fiber.Ctx) error {
		user, ok := Identity(c)
		if !ok {
			return failure(c, 401, "UNAUTHENTICATED", "Please sign in.")
		}
		err := h.service.RequireQueue(c.Context(), user, c.Params(param))
		if errors.Is(err, ErrForbidden) {
			return failure(c, 403, "FORBIDDEN", "Access denied.")
		}
		if errors.Is(err, ErrNotFound) {
			return failure(c, 404, "NOT_FOUND", "Resource not found.")
		}
		if err != nil {
			return err
		}
		return c.Next()
	}
}

// Register after Authenticate for every cookie-authenticated mutation, including
// future queue modules. Exact Origin validation also protects login from login CSRF.
func (h *HTTP) CSRF(c fiber.Ctx) error {
	if c.Get("Origin") != h.origin || !ValidCSRF(c.Cookies(h.CookieName), c.Get("X-CSRF-Token")) {
		return failure(c, 403, "CSRF_REJECTED", "Request could not be verified.")
	}
	return c.Next()
}

func (h *HTTP) me(c fiber.Ctx) error {
	user, _ := Identity(c)
	return c.JSON(fiber.Map{"user": user, "csrfToken": CSRFToken(c.Cookies(h.CookieName))})
}
func (h *HTTP) logout(c fiber.Ctx) error {
	if err := h.service.Logout(c.Context(), c.Cookies(h.CookieName)); err != nil {
		return err
	}
	h.setCookie(c, "", time.Unix(1, 0), -1)
	return c.SendStatus(204)
}

type attempts struct {
	count int
	until time.Time
}
type loginLimiter struct {
	mu      sync.Mutex
	entries map[string]attempts
	now     func() time.Time
}

func (l *loginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	current, ok := l.entries[ip]
	if !ok || !now.Before(current.until) {
		for key, value := range l.entries {
			if !now.Before(value.until) {
				delete(l.entries, key)
			}
		}
		if len(l.entries) >= 10000 {
			return false
		}
		current = attempts{until: now.Add(time.Minute)}
	}
	if current.count >= 10 {
		return false
	}
	current.count++
	l.entries[strings.Clone(ip)] = current
	return true
}
