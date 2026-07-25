package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/ikkairat/telegram-checklist-bot/internal/domain"
)

// DailyPollRepo persists daily_polls rows — one row per group message sent.
type DailyPollRepo struct {
	db *sqlx.DB
}

func NewDailyPollRepo(db *sqlx.DB) *DailyPollRepo {
	return &DailyPollRepo{db: db}
}

func (r *DailyPollRepo) Create(ctx context.Context, pollDate time.Time, messageID int64) (*domain.DailyPoll, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO daily_polls (poll_date, message_id) VALUES (?, ?)`,
		pollDate.Format("2006-01-02"), messageID)
	if err != nil {
		return nil, fmt.Errorf("create daily poll: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get new daily poll id: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *DailyPollRepo) GetByID(ctx context.Context, id int64) (*domain.DailyPoll, error) {
	var p domain.DailyPoll
	err := r.db.GetContext(ctx, &p, `SELECT * FROM daily_polls WHERE id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("get daily poll by id: %w", err)
	}
	return &p, nil
}

// GetByDate returns nil, nil if no poll exists yet for that date.
func (r *DailyPollRepo) GetByDate(ctx context.Context, pollDate time.Time) (*domain.DailyPoll, error) {
	var p domain.DailyPoll
	err := r.db.GetContext(ctx, &p,
		`SELECT * FROM daily_polls WHERE poll_date = ?`, pollDate.Format("2006-01-02"))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get daily poll by date: %w", err)
	}
	return &p, nil
}

func (r *DailyPollRepo) Close(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE daily_polls SET is_closed = 1 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("close daily poll: %w", err)
	}
	return nil
}

// CheckinRepo persists checkins rows — per-user, per-task, per-day completion marks.
type CheckinRepo struct {
	db *sqlx.DB
}

func NewCheckinRepo(db *sqlx.DB) *CheckinRepo {
	return &CheckinRepo{db: db}
}

// Toggle flips the checked state for (userID, taskID, pollDate), inserting the row if absent.
func (r *CheckinRepo) Toggle(ctx context.Context, userID, taskID int64, pollDate time.Time) (bool, error) {
	date := pollDate.Format("2006-01-02")

	var current sql.NullBool
	err := r.db.GetContext(ctx, &current,
		`SELECT checked FROM checkins WHERE user_id = ? AND task_id = ? AND poll_date = ?`,
		userID, taskID, date)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = r.db.ExecContext(ctx,
			`INSERT INTO checkins (user_id, task_id, poll_date, checked, checked_at)
			 VALUES (?, ?, ?, 1, CURRENT_TIMESTAMP)`,
			userID, taskID, date)
		if err != nil {
			return false, fmt.Errorf("insert checkin: %w", err)
		}
		return true, nil
	case err != nil:
		return false, fmt.Errorf("get checkin: %w", err)
	}

	newState := !current.Bool
	_, err = r.db.ExecContext(ctx,
		`UPDATE checkins SET checked = ?, checked_at = CASE WHEN ? THEN CURRENT_TIMESTAMP ELSE NULL END
		 WHERE user_id = ? AND task_id = ? AND poll_date = ?`,
		newState, newState, userID, taskID, date)
	if err != nil {
		return false, fmt.Errorf("update checkin: %w", err)
	}
	return newState, nil
}

func (r *CheckinRepo) ListByDate(ctx context.Context, pollDate time.Time) ([]domain.Checkin, error) {
	var checkins []domain.Checkin
	err := r.db.SelectContext(ctx, &checkins,
		`SELECT * FROM checkins WHERE poll_date = ?`, pollDate.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("list checkins by date: %w", err)
	}
	return checkins, nil
}

// ListByDateRange returns all checkins between start and end (inclusive), e.g. for a week.
func (r *CheckinRepo) ListByDateRange(ctx context.Context, start, end time.Time) ([]domain.Checkin, error) {
	var checkins []domain.Checkin
	err := r.db.SelectContext(ctx, &checkins,
		`SELECT * FROM checkins WHERE poll_date BETWEEN ? AND ?`,
		start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("list checkins by date range: %w", err)
	}
	return checkins, nil
}
