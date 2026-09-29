package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ysrckr/go-echo-template/internal/database"
	"github.com/ysrckr/go-echo-template/internal/model"
)

var (
	ErrNotFound      = errors.New("user not found")
	ErrEmailConflict = errors.New("email already in use")
)

type UserRepository struct {
	db *database.DB
}

func NewUserRepository(db *database.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]model.User, error) {
	const query = `
		SELECT id, email, name, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	users := make([]model.User, 0, limit)
	if err := r.db.SelectContext(ctx, &users, query, limit, offset); err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return users, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (model.User, error) {
	const query = `
		SELECT id, email, name, created_at, updated_at
		FROM users
		WHERE id = $1`

	var user model.User
	if err := r.db.GetContext(ctx, &user, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (r *UserRepository) Create(ctx context.Context, req model.CreateUserRequest) (model.User, error) {
	const query = `
		INSERT INTO users (email, name)
		VALUES (:email, :name)
		RETURNING id, email, name, created_at, updated_at`

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return model.User{}, fmt.Errorf("prepare create user: %w", err)
	}
	defer stmt.Close()

	var user model.User
	if err := stmt.GetContext(ctx, &user, req); err != nil {
		if isUniqueViolation(err) {
			return model.User{}, ErrEmailConflict
		}
		return model.User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (r *UserRepository) Update(ctx context.Context, id uuid.UUID, req model.UpdateUserRequest) (model.User, error) {
	const query = `
		UPDATE users
		SET email = $2, name = $3, updated_at = now()
		WHERE id = $1
		RETURNING id, email, name, created_at, updated_at`

	var user model.User
	if err := r.db.GetContext(ctx, &user, query, id, req.Email, req.Name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return model.User{}, ErrEmailConflict
		}
		return model.User{}, fmt.Errorf("update user: %w", err)
	}
	return user, nil
}

func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const query = `DELETE FROM users WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete user rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
