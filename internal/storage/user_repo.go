package storage

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/ikkairat/telegram-checklist-bot/internal/domain"
)

type UserRepo struct {
	db *sqlx.DB
}

func NewUserRepo(db *sqlx.DB) *UserRepo {
	return &UserRepo{db: db}
}

// Upsert creates the user if telegram_id is new, or updates username/
// full_name otherwise. It also reactivates the user (is_active = 1) on
// conflict — since nothing ever deactivates a user except SetActive(false)
// (e.g. an admin marking someone as having left the group), running /start
// again is a legitimate "I'm back" signal and should undo that. It never
// touches is_admin — new rows get the schema default (non-admin); use
// SetAdmin to change that.
func (r *UserRepo) Upsert(ctx context.Context, u *domain.User) error {
	const q = `
		INSERT INTO users (telegram_id, username, full_name)
		VALUES (:telegram_id, :username, :full_name)
		ON CONFLICT(telegram_id) DO UPDATE SET
			username = excluded.username,
			full_name = excluded.full_name,
			is_active = 1
	`
	_, err := r.db.NamedExecContext(ctx, q, u)
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}
	return nil
}

func (r *UserRepo) GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	var u domain.User
	err := r.db.GetContext(ctx, &u, `SELECT * FROM users WHERE telegram_id = ?`, telegramID)
	if err != nil {
		return nil, fmt.Errorf("get user by telegram_id: %w", err)
	}
	return &u, nil
}

// GetByUsername looks up an active user by their Telegram @username
// (case-insensitive, without the leading @).
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	var u domain.User
	err := r.db.GetContext(ctx, &u,
		`SELECT * FROM users WHERE username = ? COLLATE NOCASE AND is_active = 1`, username)
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return &u, nil
}

func (r *UserRepo) ListActive(ctx context.Context) ([]domain.User, error) {
	var users []domain.User
	err := r.db.SelectContext(ctx, &users, `SELECT * FROM users WHERE is_active = 1 ORDER BY full_name`)
	if err != nil {
		return nil, fmt.Errorf("list active users: %w", err)
	}
	return users, nil
}

func (r *UserRepo) SetActive(ctx context.Context, telegramID int64, active bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET is_active = ? WHERE telegram_id = ?`, active, telegramID)
	if err != nil {
		return fmt.Errorf("set user active: %w", err)
	}
	return nil
}

func (r *UserRepo) SetAdmin(ctx context.Context, telegramID int64, admin bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET is_admin = ? WHERE telegram_id = ?`, admin, telegramID)
	if err != nil {
		return fmt.Errorf("set user admin: %w", err)
	}
	return nil
}
