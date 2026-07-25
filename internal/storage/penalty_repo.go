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

// WeekRepo persists weeks rows — one row per weekly penalty period.
type WeekRepo struct {
	db *sqlx.DB
}

func NewWeekRepo(db *sqlx.DB) *WeekRepo {
	return &WeekRepo{db: db}
}

func (r *WeekRepo) Create(ctx context.Context, start, end time.Time, penaltyAmount int64) (*domain.Week, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO weeks (start_date, end_date, penalty_amount) VALUES (?, ?, ?)`,
		start.Format("2006-01-02"), end.Format("2006-01-02"), penaltyAmount)
	if err != nil {
		return nil, fmt.Errorf("create week: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get new week id: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *WeekRepo) GetByID(ctx context.Context, id int64) (*domain.Week, error) {
	var w domain.Week
	err := r.db.GetContext(ctx, &w, `SELECT * FROM weeks WHERE id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("get week by id: %w", err)
	}
	return &w, nil
}

// GetOpen returns the current not-yet-closed week, or nil, nil if none exists.
func (r *WeekRepo) GetOpen(ctx context.Context) (*domain.Week, error) {
	var w domain.Week
	err := r.db.GetContext(ctx, &w, `SELECT * FROM weeks WHERE is_closed = 0 ORDER BY start_date DESC LIMIT 1`)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get open week: %w", err)
	}
	return &w, nil
}

// GetLastClosed returns the most recently closed week, or nil if none exists.
func (r *WeekRepo) GetLastClosed(ctx context.Context) (*domain.Week, error) {
	var w domain.Week
	err := r.db.GetContext(ctx, &w, `SELECT * FROM weeks WHERE is_closed = 1 ORDER BY end_date DESC LIMIT 1`)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get last closed week: %w", err)
	}
	return &w, nil
}

func (r *WeekRepo) SetReportPDFPath(ctx context.Context, id int64, path string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE weeks SET report_pdf_path = ? WHERE id = ?`, path, id)
	if err != nil {
		return fmt.Errorf("set week report pdf path: %w", err)
	}
	return nil
}

func (r *WeekRepo) SetPenaltyAmount(ctx context.Context, id, amount int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE weeks SET penalty_amount = ? WHERE id = ?`, amount, id)
	if err != nil {
		return fmt.Errorf("set week penalty amount: %w", err)
	}
	return nil
}

func (r *WeekRepo) Close(ctx context.Context, id int64, reportPDFPath string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE weeks SET is_closed = 1, report_pdf_path = ? WHERE id = ?`, reportPDFPath, id)
	if err != nil {
		return fmt.Errorf("close week: %w", err)
	}
	return nil
}

// PenaltyRepo persists penalties rows — one row per (week, user).
type PenaltyRepo struct {
	db *sqlx.DB
}

func NewPenaltyRepo(db *sqlx.DB) *PenaltyRepo {
	return &PenaltyRepo{db: db}
}

func (r *PenaltyRepo) Upsert(ctx context.Context, p *domain.Penalty) error {
	const q = `
		INSERT INTO penalties (week_id, user_id, total_tasks, missed_tasks, amount)
		VALUES (:week_id, :user_id, :total_tasks, :missed_tasks, :amount)
		ON CONFLICT(week_id, user_id) DO UPDATE SET
			total_tasks = excluded.total_tasks,
			missed_tasks = excluded.missed_tasks,
			amount = excluded.amount
	`
	_, err := r.db.NamedExecContext(ctx, q, p)
	if err != nil {
		return fmt.Errorf("upsert penalty: %w", err)
	}
	return nil
}

func (r *PenaltyRepo) ListByWeek(ctx context.Context, weekID int64) ([]domain.Penalty, error) {
	var penalties []domain.Penalty
	err := r.db.SelectContext(ctx, &penalties, `SELECT * FROM penalties WHERE week_id = ?`, weekID)
	if err != nil {
		return nil, fmt.Errorf("list penalties by week: %w", err)
	}
	return penalties, nil
}

func (r *PenaltyRepo) MarkPaid(ctx context.Context, weekID, userID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE penalties SET is_paid = 1 WHERE week_id = ? AND user_id = ?`, weekID, userID)
	if err != nil {
		return fmt.Errorf("mark penalty paid: %w", err)
	}
	return nil
}

// GetByWeekAndUser returns nil, nil if the user has no penalty row for that week.
func (r *PenaltyRepo) GetByWeekAndUser(ctx context.Context, weekID, userID int64) (*domain.Penalty, error) {
	var p domain.Penalty
	err := r.db.GetContext(ctx, &p,
		`SELECT * FROM penalties WHERE week_id = ? AND user_id = ?`, weekID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get penalty by week and user: %w", err)
	}
	return &p, nil
}

// Forgive zeroes out a penalty's amount and marks it settled (nothing left
// to pay), used when an admin waives a fine instead of collecting it.
func (r *PenaltyRepo) Forgive(ctx context.Context, weekID, userID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE penalties SET amount = 0, is_paid = 1 WHERE week_id = ? AND user_id = ?`, weekID, userID)
	if err != nil {
		return fmt.Errorf("forgive penalty: %w", err)
	}
	return nil
}

// FundLedgerRepo persists fund_ledger rows — the running penalty fund.
type FundLedgerRepo struct {
	db *sqlx.DB
}

func NewFundLedgerRepo(db *sqlx.DB) *FundLedgerRepo {
	return &FundLedgerRepo{db: db}
}

func (r *FundLedgerRepo) Add(ctx context.Context, weekID int64, amount int64, note string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO fund_ledger (week_id, amount, note) VALUES (?, ?, ?)`, weekID, amount, note)
	if err != nil {
		return fmt.Errorf("add fund ledger entry: %w", err)
	}
	return nil
}

func (r *FundLedgerRepo) Total(ctx context.Context) (int64, error) {
	var total sql.NullInt64
	err := r.db.GetContext(ctx, &total, `SELECT SUM(amount) FROM fund_ledger`)
	if err != nil {
		return 0, fmt.Errorf("get fund total: %w", err)
	}
	return total.Int64, nil
}
