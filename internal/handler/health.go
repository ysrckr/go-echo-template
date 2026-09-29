package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/yasar/go-echo-template/internal/config"
	"github.com/yasar/go-echo-template/internal/database"
)

type Health struct {
	db  *database.DB
	app config.App
}

func NewHealth(db *database.DB, app config.App) *Health {
	return &Health{db: db, app: app}
}

type healthResponse struct {
	Status   string `json:"status"`
	Service  string `json:"service"`
	Version  string `json:"version"`
	Database string `json:"database,omitempty"`
}

// Live reports that the process is up. It deliberately touches no dependency,
// so a slow database never triggers a restart.
func (h *Health) Live(c *echo.Context) error {
	return c.JSON(http.StatusOK, healthResponse{
		Status:  "ok",
		Service: h.app.Name,
		Version: h.app.Version,
	})
}

// Ready reports whether the service can serve traffic, database included.
func (h *Health) Ready(c *echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Health(ctx); err != nil {
		return c.JSON(http.StatusServiceUnavailable, healthResponse{
			Status:   "unavailable",
			Service:  h.app.Name,
			Version:  h.app.Version,
			Database: "down",
		})
	}

	return c.JSON(http.StatusOK, healthResponse{
		Status:   "ok",
		Service:  h.app.Name,
		Version:  h.app.Version,
		Database: "up",
	})
}
