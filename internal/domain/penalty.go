package domain

import "time"

// Week — неделя, за которую агрегируются штрафы.
type Week struct {
	ID            int64     `db:"id"`
	StartDate     time.Time `db:"start_date"`
	EndDate       time.Time `db:"end_date"`
	PenaltyAmount int64     `db:"penalty_amount"` // ставка за 1 пропуск, в сомах
	IsClosed      bool      `db:"is_closed"`
	ReportPDFPath string    `db:"report_pdf_path"`
}

// Penalty — итоговый штраф участника за неделю.
type Penalty struct {
	ID          int64 `db:"id"`
	WeekID      int64 `db:"week_id"`
	UserID      int64 `db:"user_id"`
	TotalTasks  int   `db:"total_tasks"`
	MissedTasks int   `db:"missed_tasks"`
	Amount      int64 `db:"amount"`
	IsPaid      bool  `db:"is_paid"`
}

// FundLedgerEntry — запись в общем накопительном фонде.
type FundLedgerEntry struct {
	ID        int64     `db:"id"`
	WeekID    int64     `db:"week_id"`
	Amount    int64     `db:"amount"`
	Note      string    `db:"note"`
	CreatedAt time.Time `db:"created_at"`
}
