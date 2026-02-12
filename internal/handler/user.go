package handler

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"gofiberobservability/pkg/database"
	"gofiberobservability/pkg/logger"

	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// User represents a user row.
type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateUserRequest is the request body for creating a user.
type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ListUsers returns all users from the database with pagination support.
func ListUsers(serviceName string) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx := c.Context()
		log := logger.GetLoggerWithTraceContext(ctx)

		// Pagination parameters (Manual parse for Fiber v3)
		limit, _ := strconv.Atoi(c.Query("limit", "10"))
		if limit > 100 {
			limit = 100
		}
		page, _ := strconv.Atoi(c.Query("page", "1"))
		if page < 1 {
			page = 1
		}
		offset := (page - 1) * limit

		tr := otel.Tracer(serviceName)
		ctx, span := tr.Start(ctx, "db.list-users")
		defer span.End()

		span.SetAttributes(
			attribute.Int("pagination.limit", limit),
			attribute.Int("pagination.page", page),
		)

		rows, err := database.GetPool().Query(ctx,
			"SELECT id, name, email, created_at FROM users ORDER BY id LIMIT $1 OFFSET $2",
			limit, offset,
		)
		if err != nil {
			log.Error("Failed to query users", zap.Error(err))
			return fiber.NewError(fiber.StatusInternalServerError, "Failed to fetch users")
		}
		defer rows.Close()

		users := make([]User, 0)
		for rows.Next() {
			var u User
			if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt); err != nil {
				log.Error("Failed to scan user row", zap.Error(err))
				continue
			}
			users = append(users, u)
		}

		log.Info("Users fetched with pagination",
			zap.Int("count", len(users)),
			zap.Int("limit", limit),
			zap.Int("page", page),
		)

		return c.JSON(fiber.Map{
			"users": users,
			"metadata": fiber.Map{
				"count": len(users),
				"limit": limit,
				"page":  page,
			},
		})
	}
}

// CreateUser inserts a new user into the database.
func CreateUser(serviceName string) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx := c.Context()
		log := logger.GetLoggerWithTraceContext(ctx)

		var req CreateUserRequest
		if err := c.Bind().JSON(&req); err != nil {
			log.Error("Invalid request body", zap.Error(err))
			return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
		}

		if req.Name == "" || req.Email == "" {
			return fiber.NewError(fiber.StatusBadRequest, "name and email are required")
		}

		tr := otel.Tracer(serviceName)
		ctx, span := tr.Start(ctx, "db.create-user")
		defer span.End()

		var user User
		err := database.GetPool().QueryRow(ctx,
			"INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id, name, email, created_at",
			req.Name, req.Email,
		).Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
		if err != nil {
			log.Error("Failed to create user", zap.Error(err))
			return fiber.NewError(fiber.StatusInternalServerError, "Failed to create user")
		}

		span.SetAttributes(attribute.Int("user.id", user.ID))
		log.Info("User created", zap.Int("id", user.ID), zap.String("email", user.Email))

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"message": "User created",
			"user":    user,
		})
	}
}

// GetUser returns a single user by ID with Redis caching.
func GetUser(serviceName string) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx := c.Context()
		log := logger.GetLoggerWithTraceContext(ctx)

		id := c.Params("id")
		cacheKey := fmt.Sprintf("user:%s", id)

		tr := otel.Tracer(serviceName)
		ctx, span := tr.Start(ctx, "handler.get-user")
		defer span.End()

		span.SetAttributes(attribute.String("user.id", id))

		// 1. Try to get from Redis
		val, err := database.GetRedis().Get(ctx, cacheKey).Result()
		if err == nil {
			// Cache Hit
			var user User
			if err := json.Unmarshal([]byte(val), &user); err == nil {
				log.Info("Cache hit", zap.String("id", id))
				span.SetAttributes(attribute.Bool("cache.hit", true))
				return c.JSON(user)
			}
			log.Warn("Failed to unmarshal cached user", zap.Error(err))
		}

		// 2. Cache Miss - Get from Database
		log.Info("Cache miss", zap.String("id", id))
		span.SetAttributes(attribute.Bool("cache.hit", false))

		var user User
		err = database.GetPool().QueryRow(ctx,
			"SELECT id, name, email, created_at FROM users WHERE id = $1", id,
		).Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
		if err != nil {
			log.Error("User not found", zap.String("id", id), zap.Error(err))
			return fiber.NewError(fiber.StatusNotFound, "User not found")
		}

		// 3. Save to Redis
		userJSON, _ := json.Marshal(user)
		database.GetRedis().Set(ctx, cacheKey, userJSON, 10*time.Minute)

		return c.JSON(user)
	}
}

// DeleteUser deletes a user by ID.
func DeleteUser(serviceName string) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx := c.Context()
		log := logger.GetLoggerWithTraceContext(ctx)

		id := c.Params("id")

		tr := otel.Tracer(serviceName)
		ctx, span := tr.Start(ctx, "db.delete-user")
		defer span.End()

		tag, err := database.GetPool().Exec(ctx, "DELETE FROM users WHERE id = $1", id)
		if err != nil {
			log.Error("Failed to delete user", zap.String("id", id), zap.Error(err))
			return fiber.NewError(fiber.StatusInternalServerError, "Failed to delete user")
		}

		if tag.RowsAffected() == 0 {
			return fiber.NewError(fiber.StatusNotFound, "User not found")
		}

		log.Info("User deleted", zap.String("id", id))

		return c.JSON(fiber.Map{
			"message": "User deleted",
		})
	}
}
