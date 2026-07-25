package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ikkairat/telegram-checklist-bot/internal/domain"
	"github.com/ikkairat/telegram-checklist-bot/internal/storage"
)

// PenaltyService computes and persists weekly penalties. Formula (variant
// A, fixed in the architecture doc §5): amount = missed_checks * rate,
// where rate is weeks.penalty_amount, set by an admin via /setpenalty.
type PenaltyService struct {
	users     *storage.UserRepo
	tasks     *storage.TaskRepo
	checkins  *storage.CheckinRepo
	weeks     *storage.WeekRepo
	penalties *storage.PenaltyRepo
	fund      *storage.FundLedgerRepo

	reportWeekday time.Weekday
}

func NewPenaltyService(
	users *storage.UserRepo,
	tasks *storage.TaskRepo,
	checkins *storage.CheckinRepo,
	weeks *storage.WeekRepo,
	penalties *storage.PenaltyRepo,
	fund *storage.FundLedgerRepo,
	reportWeekday time.Weekday,
) *PenaltyService {
	return &PenaltyService{
		users: users, tasks: tasks, checkins: checkins,
		weeks: weeks, penalties: penalties, fund: fund,
		reportWeekday: reportWeekday,
	}
}

// WeekRange returns the Monday-to-reportWeekday span that contains `now`,
// e.g. for reportWeekday=Sunday and any day this week, it returns that
// week's Monday..Sunday range (as midnight-truncated dates).
func WeekRange(now time.Time, reportWeekday time.Weekday) (start, end time.Time) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	daysUntilEnd := (int(reportWeekday) - int(today.Weekday()) + 7) % 7
	end = today.AddDate(0, 0, daysUntilEnd)
	start = end.AddDate(0, 0, -6)
	return start, end
}

// GetOpenWeek returns the current not-yet-closed week, or nil if none exists
// (e.g. an admin has never run /setpenalty yet).
func (s *PenaltyService) GetOpenWeek(ctx context.Context) (*domain.Week, error) {
	return s.weeks.GetOpen(ctx)
}

// EnsureCurrentWeek returns the open week, creating one covering `now` with
// defaultPenaltyAmount as its rate if none exists yet.
func (s *PenaltyService) EnsureCurrentWeek(ctx context.Context, now time.Time, defaultPenaltyAmount int64) (*domain.Week, error) {
	week, err := s.weeks.GetOpen(ctx)
	if err != nil {
		return nil, err
	}
	if week != nil {
		return week, nil
	}
	start, end := WeekRange(now, s.reportWeekday)
	return s.weeks.Create(ctx, start, end, defaultPenaltyAmount)
}

// SetPenaltyRate sets the per-miss penalty rate (in soms) for the current
// week, creating the week if this is the first time it's been set.
func (s *PenaltyService) SetPenaltyRate(ctx context.Context, now time.Time, amount int64) (*domain.Week, error) {
	week, err := s.EnsureCurrentWeek(ctx, now, amount)
	if err != nil {
		return nil, err
	}
	if week.PenaltyAmount == amount {
		return week, nil
	}
	if err := s.weeks.SetPenaltyAmount(ctx, week.ID, amount); err != nil {
		return nil, err
	}
	week.PenaltyAmount = amount
	return week, nil
}

// UserPenalty pairs a computed Penalty with the user it belongs to, for
// rendering (report text, PDF).
type UserPenalty struct {
	User    domain.User
	Penalty domain.Penalty
}

// weekTaskStats computes, for the date range [start, end], the total task
// slots (active tasks × days) and how many each active user completed.
func (s *PenaltyService) weekTaskStats(ctx context.Context, start, end time.Time) (users []domain.User, totalTasks int, completedByUser map[int64]int, err error) {
	tasks, err := s.tasks.ListActive(ctx)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("list active tasks: %w", err)
	}
	users, err = s.users.ListActive(ctx)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("list active users: %w", err)
	}
	checkins, err := s.checkins.ListByDateRange(ctx, start, end)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("list checkins for range: %w", err)
	}

	days := int(end.Sub(start).Hours()/24) + 1
	totalTasks = len(tasks) * days

	completedByUser = make(map[int64]int, len(users))
	for _, c := range checkins {
		if c.Checked {
			completedByUser[c.UserID]++
		}
	}
	return users, totalTasks, completedByUser, nil
}

// CloseWeek computes each active user's missed-task count and penalty
// amount over week's date range, persists penalties and a fund_ledger
// entry, and marks the week closed. It returns the per-user breakdown and
// the total added to the fund.
func (s *PenaltyService) CloseWeek(ctx context.Context, week *domain.Week) ([]UserPenalty, int64, error) {
	users, totalTasks, completedByUser, err := s.weekTaskStats(ctx, week.StartDate, week.EndDate)
	if err != nil {
		return nil, 0, err
	}

	results := make([]UserPenalty, 0, len(users))
	var fundTotal int64
	for _, u := range users {
		missed := totalTasks - completedByUser[u.ID]
		if missed < 0 {
			missed = 0
		}
		amount := int64(missed) * week.PenaltyAmount

		p := domain.Penalty{
			WeekID:      week.ID,
			UserID:      u.ID,
			TotalTasks:  totalTasks,
			MissedTasks: missed,
			Amount:      amount,
		}
		if err := s.penalties.Upsert(ctx, &p); err != nil {
			return nil, 0, fmt.Errorf("upsert penalty for user %d: %w", u.ID, err)
		}

		results = append(results, UserPenalty{User: u, Penalty: p})
		fundTotal += amount
	}

	if fundTotal > 0 {
		if err := s.fund.Add(ctx, week.ID, fundTotal, "еженедельные штрафы"); err != nil {
			return nil, 0, fmt.Errorf("add fund ledger entry: %w", err)
		}
	}

	if err := s.weeks.Close(ctx, week.ID, ""); err != nil {
		return nil, 0, fmt.Errorf("close week: %w", err)
	}

	return results, fundTotal, nil
}

// StartNextWeek creates the week immediately following closedWeek, carrying
// over its penalty rate.
func (s *PenaltyService) StartNextWeek(ctx context.Context, closedWeek *domain.Week) (*domain.Week, error) {
	start := closedWeek.EndDate.AddDate(0, 0, 1)
	end := start.AddDate(0, 0, 6)
	return s.weeks.Create(ctx, start, end, closedWeek.PenaltyAmount)
}

// FundTotal returns the running total of the penalty fund.
func (s *PenaltyService) FundTotal(ctx context.Context) (int64, error) {
	return s.fund.Total(ctx)
}

// MarkPaid marks a user's penalty for a week as paid.
func (s *PenaltyService) MarkPaid(ctx context.Context, weekID, userID int64) error {
	return s.penalties.MarkPaid(ctx, weekID, userID)
}

// GetLastClosedWeek returns the most recently closed week (the one whose
// penalties are currently being paid off), or nil if none has closed yet.
func (s *PenaltyService) GetLastClosedWeek(ctx context.Context) (*domain.Week, error) {
	return s.weeks.GetLastClosed(ctx)
}

// FindUserByUsername resolves a Telegram @username to a user, for /mark_paid.
func (s *PenaltyService) FindUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	return s.users.GetByUsername(ctx, username)
}

// SetReportPath records where the generated PDF for a (now closed) week was
// saved, so it can be looked up again later.
func (s *PenaltyService) SetReportPath(ctx context.Context, weekID int64, path string) error {
	return s.weeks.SetReportPDFPath(ctx, weekID, path)
}

// ErrNoPenalty is returned by Forgive when the user has no penalty row for
// the given week (e.g. they weren't an active member that week).
var ErrNoPenalty = fmt.Errorf("no penalty found for that user in that week")

// Forgive waives a user's penalty for a week: it zeroes their amount,
// marks it settled, and removes the waived sum from the fund so the total
// stays consistent with what's actually still owed. It returns the amount
// that was forgiven.
func (s *PenaltyService) Forgive(ctx context.Context, weekID, userID int64) (int64, error) {
	p, err := s.penalties.GetByWeekAndUser(ctx, weekID, userID)
	if err != nil {
		return 0, err
	}
	if p == nil {
		return 0, ErrNoPenalty
	}
	if p.Amount == 0 {
		return 0, nil
	}

	if err := s.penalties.Forgive(ctx, weekID, userID); err != nil {
		return 0, err
	}
	if err := s.fund.Add(ctx, weekID, -p.Amount, "штраф прощён администратором"); err != nil {
		return 0, fmt.Errorf("adjust fund for forgiven penalty: %w", err)
	}
	return p.Amount, nil
}

// UserWeekStatus reports a single user's completion so far in the current
// week (open week if one exists, otherwise the calendar week containing
// now), for the /report command.
func (s *PenaltyService) UserWeekStatus(ctx context.Context, now time.Time, userID int64) (start, end time.Time, totalTasks, completed, missed int, err error) {
	week, err := s.weeks.GetOpen(ctx)
	if err != nil {
		return time.Time{}, time.Time{}, 0, 0, 0, err
	}
	if week != nil {
		start, end = week.StartDate, week.EndDate
	} else {
		start, end = WeekRange(now, s.reportWeekday)
	}

	_, totalTasks, completedByUser, err := s.weekTaskStats(ctx, start, end)
	if err != nil {
		return time.Time{}, time.Time{}, 0, 0, 0, err
	}

	completed = completedByUser[userID]
	missed = totalTasks - completed
	if missed < 0 {
		missed = 0
	}
	return start, end, totalTasks, completed, missed, nil
}
