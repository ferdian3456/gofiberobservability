package handler

import (
	"gofiberobservability/pkg/database"

	"github.com/gofiber/fiber/v3"
)

// HealthCheck returns the health status of the application and its dependencies.
func HealthCheck() fiber.Handler {
	return func(c fiber.Ctx) error {
		dbStatus := "up"
		if err := database.HealthCheck(c.Context()); err != nil {
			dbStatus = "down"
		}

		redisStatus := "up"
		if err := database.RedisHealthCheck(c.Context()); err != nil {
			redisStatus = "down"
		}

		status := fiber.StatusOK
		if dbStatus != "up" || redisStatus != "up" {
			status = fiber.StatusServiceUnavailable
		}

		return c.Status(status).JSON(fiber.Map{
			"status": func() string {
				if status == fiber.StatusOK {
					return "healthy"
				}
				return "unhealthy"
			}(),
			"dependencies": fiber.Map{
				"database": dbStatus,
				"redis":    redisStatus,
			},
		})
	}
}
