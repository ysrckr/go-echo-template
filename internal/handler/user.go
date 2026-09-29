package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"

	"github.com/ysrckr/go-echo-template/internal/model"
	"github.com/ysrckr/go-echo-template/internal/repository"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

type User struct {
	repo *repository.UserRepository
}

func NewUser(repo *repository.UserRepository) *User {
	return &User{repo: repo}
}

func (h *User) List(c *echo.Context) error {
	// v5's generic helpers parse and default in one step.
	limit, err := echo.QueryParamOr[int](c, "limit", defaultLimit)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "limit must be an integer")
	}
	offset, err := echo.QueryParamOr[int](c, "offset", 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "offset must be an integer")
	}

	if limit < 1 || limit > maxLimit {
		limit = defaultLimit
	}
	if offset < 0 {
		offset = 0
	}

	users, err := h.repo.List(c.Request().Context(), limit, offset)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, users)
}

func (h *User) Get(c *echo.Context) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}

	user, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return mapRepoError(err)
	}
	return c.JSON(http.StatusOK, user)
}

func (h *User) Create(c *echo.Context) error {
	var req model.CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	user, err := h.repo.Create(c.Request().Context(), req)
	if err != nil {
		return mapRepoError(err)
	}
	return c.JSON(http.StatusCreated, user)
}

func (h *User) Update(c *echo.Context) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}

	var req model.UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	user, err := h.repo.Update(c.Request().Context(), id, req)
	if err != nil {
		return mapRepoError(err)
	}
	return c.JSON(http.StatusOK, user)
}

func (h *User) Delete(c *echo.Context) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}

	if err := h.repo.Delete(c.Request().Context(), id); err != nil {
		return mapRepoError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func parseID(c *echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, echo.NewHTTPError(http.StatusBadRequest, "id must be a uuid")
	}
	return id, nil
}

// mapRepoError translates repository errors into HTTP errors, keeping storage
// concerns out of the handler bodies.
func mapRepoError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	case errors.Is(err, repository.ErrEmailConflict):
		return echo.NewHTTPError(http.StatusConflict, "email already in use")
	default:
		return err
	}
}
