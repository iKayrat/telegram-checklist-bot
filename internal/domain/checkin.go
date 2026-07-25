package domain

import "time"

// DailyPoll — одна ежедневная рассылка чек-листа (одно сообщение в группе).
type DailyPoll struct {
	ID        int64     `db:"id"`
	PollDate  time.Time `db:"poll_date"`
	MessageID int64     `db:"message_id"`
	IsClosed  bool      `db:"is_closed"`
	CreatedAt time.Time `db:"created_at"`
}

// Checkin — отметка о выполнении задачи пользователем в конкретный день.
type Checkin struct {
	ID        int64      `db:"id"`
	UserID    int64      `db:"user_id"`
	TaskID    int64      `db:"task_id"`
	PollDate  time.Time  `db:"poll_date"`
	Checked   bool       `db:"checked"`
	CheckedAt *time.Time `db:"checked_at"`
}
