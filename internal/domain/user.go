package domain

import "time"

// User — участник группы.
type User struct {
	ID         int64     `db:"id"`
	TelegramID int64     `db:"telegram_id"`
	Username   string    `db:"username"`
	FullName   string    `db:"full_name"`
	IsAdmin    bool      `db:"is_admin"`
	IsActive   bool      `db:"is_active"`
	CreatedAt  time.Time `db:"created_at"`
}
