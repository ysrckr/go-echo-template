package server

import (
	"github.com/labstack/echo/v5"

	"github.com/ysrckr/go-echo-template/internal/handler"
)

func registerRoutes(e *echo.Echo, health *handler.Health, users *handler.User) {
	e.GET("/health/live", health.Live)
	e.GET("/health/ready", health.Ready)

	v1 := e.Group("/api/v1")

	v1.GET("/users", users.List)
	v1.POST("/users", users.Create)
	v1.GET("/users/:id", users.Get)
	v1.PUT("/users/:id", users.Update)
	v1.DELETE("/users/:id", users.Delete)
}
