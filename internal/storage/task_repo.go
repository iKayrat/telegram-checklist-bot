package storage

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/ikkairat/telegram-checklist-bot/internal/domain"
)

type TaskRepo struct {
	db *sqlx.DB
}

func NewTaskRepo(db *sqlx.DB) *TaskRepo {
	return &TaskRepo{db: db}
}

func (r *TaskRepo) Create(ctx context.Context, title string, sortOrder int) (*domain.Task, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO tasks (title, sort_order) VALUES (?, ?)`, title, sortOrder)
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get new task id: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *TaskRepo) GetByID(ctx context.Context, id int64) (*domain.Task, error) {
	var t domain.Task
	err := r.db.GetContext(ctx, &t, `SELECT * FROM tasks WHERE id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("get task by id: %w", err)
	}
	return &t, nil
}

func (r *TaskRepo) ListActive(ctx context.Context) ([]domain.Task, error) {
	var tasks []domain.Task
	err := r.db.SelectContext(ctx, &tasks,
		`SELECT * FROM tasks WHERE is_active = 1 ORDER BY sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("list active tasks: %w", err)
	}
	return tasks, nil
}

func (r *TaskRepo) Deactivate(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tasks SET is_active = 0 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deactivate task: %w", err)
	}
	return nil
}
