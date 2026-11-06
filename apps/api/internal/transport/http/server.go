package http

import (
	"errors"
	"time"

	"dinequeue.local/api/internal/health"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

func New(service *health.Service) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName: "DineQueue", BodyLimit: 1 << 20,
		ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			status, code, message := 500, "INTERNAL_ERROR", "Request could not be processed."
			var httpError *fiber.Error
			if errors.As(err, &httpError) && httpError.Code == 404 {
				status, code, message = 404, "NOT_FOUND", "Resource not found."
			} else if errors.As(err, &httpError) && httpError.Code >= 400 && httpError.Code < 500 {
				status, code, message = httpError.Code, "REQUEST_REJECTED", "Request was rejected."
			}
			return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": message, "requestId": c.GetRespHeader("X-Request-ID")}})
		},
	})
	app.Use(requestid.New())
	app.Use(recover.New())
	app.Use(func(c fiber.Ctx) error {
		c.Set("Cache-Control", "no-store")
		c.Set("X-Content-Type-Options", "nosniff")
		return c.Next()
	})
	app.Get("/health/live", func(c fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok"}) })
	app.Get("/health/ready", func(c fiber.Ctx) error {
		if err := service.Ready(c.Context()); err != nil {
			return c.Status(503).JSON(fiber.Map{"status": "unavailable"})
		}
		return c.JSON(fiber.Map{"status": "ready"})
	})
	return app
}
