package domain

import "time"

// Task — пункт чек-листа, общий для всех участников.
type Task struct {
	ID        int64     `db:"id"`
	Title     string    `db:"title"`
	IsActive  bool      `db:"is_active"`
	SortOrder int       `db:"sort_order"`
	CreatedAt time.Time `db:"created_at"`
}
