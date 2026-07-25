package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ikkairat/telegram-checklist-bot/internal/domain"
	"github.com/ikkairat/telegram-checklist-bot/internal/storage"
)

// ChecklistService contains the daily-checklist business logic: it is the
// only layer that knows both about Telegram-shaped inputs (telegram IDs)
// and about the storage repos. Bot handlers call this instead of touching
// storage directly.
type ChecklistService struct {
	users    *storage.UserRepo
	tasks    *storage.TaskRepo
	polls    *storage.DailyPollRepo
	checkins *storage.CheckinRepo
}

func NewChecklistService(
	users *storage.UserRepo,
	tasks *storage.TaskRepo,
	polls *storage.DailyPollRepo,
	checkins *storage.CheckinRepo,
) *ChecklistService {
	return &ChecklistService{users: users, tasks: tasks, polls: polls, checkins: checkins}
}

// RegisterUser records a user the first time they interact with the bot
// (typically /start), so the bot can later DM them reminders.
func (s *ChecklistService) RegisterUser(ctx context.Context, telegramID int64, username, fullName string) (*domain.User, error) {
	u := &domain.User{TelegramID: telegramID, Username: username, FullName: fullName}
	if err := s.users.Upsert(ctx, u); err != nil {
		return nil, err
	}
	return s.users.GetByTelegramID(ctx, telegramID)
}

func (s *ChecklistService) AddTask(ctx context.Context, title string) (*domain.Task, error) {
	active, err := s.tasks.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active tasks: %w", err)
	}
	return s.tasks.Create(ctx, title, len(active))
}

func (s *ChecklistService) RemoveTask(ctx context.Context, taskID int64) error {
	return s.tasks.Deactivate(ctx, taskID)
}

func (s *ChecklistService) ActiveTasks(ctx context.Context) ([]domain.Task, error) {
	return s.tasks.ListActive(ctx)
}

// GetPoll returns the poll for date, or nil if the checklist hasn't been posted yet.
func (s *ChecklistService) GetPoll(ctx context.Context, date time.Time) (*domain.DailyPoll, error) {
	return s.polls.GetByDate(ctx, date)
}

func (s *ChecklistService) CreatePoll(ctx context.Context, date time.Time, messageID int64) (*domain.DailyPoll, error) {
	return s.polls.Create(ctx, date, messageID)
}

// ClosePoll marks a daily poll as closed, meaning any task left unchecked
// for that day now permanently counts as missed.
func (s *ChecklistService) ClosePoll(ctx context.Context, pollID int64) error {
	return s.polls.Close(ctx, pollID)
}

// ToggleCheckin flips whether telegramID has completed taskID on date, and
// returns the new state. The caller (bot handler) must have already
// confirmed the user is registered.
func (s *ChecklistService) ToggleCheckin(ctx context.Context, telegramID, taskID int64, date time.Time) (bool, error) {
	user, err := s.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return false, fmt.Errorf("resolve user: %w", err)
	}
	return s.checkins.Toggle(ctx, user.ID, taskID, date)
}

// FindUserByUsername resolves a Telegram @username to a registered user.
func (s *ChecklistService) FindUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	return s.users.GetByUsername(ctx, username)
}

// IsAdmin reports whether a registered user has been granted admin rights
// via /setadmin. Unregistered users are treated as non-admins rather than
// an error.
func (s *ChecklistService) IsAdmin(ctx context.Context, telegramID int64) (bool, error) {
	u, err := s.users.GetByTelegramID(ctx, telegramID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return u.IsAdmin, nil
}

// SetAdmin grants or revokes a registered user's admin rights.
func (s *ChecklistService) SetAdmin(ctx context.Context, telegramID int64, admin bool) error {
	return s.users.SetAdmin(ctx, telegramID, admin)
}

// UserStatus is one row of the checklist view: a user and which of the
// day's tasks they have completed so far.
type UserStatus struct {
	User domain.User
	Done map[int64]bool // taskID -> checked
}

// BuildStatuses returns the current completion state of every active user
// for date, used to render both the checklist message text and, later,
// weekly reports.
func (s *ChecklistService) BuildStatuses(ctx context.Context, date time.Time) ([]UserStatus, error) {
	users, err := s.users.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active users: %w", err)
	}

	checkins, err := s.checkins.ListByDate(ctx, date)
	if err != nil {
		return nil, fmt.Errorf("list checkins: %w", err)
	}

	doneByUser := make(map[int64]map[int64]bool, len(users))
	for _, c := range checkins {
		if !c.Checked {
			continue
		}
		if doneByUser[c.UserID] == nil {
			doneByUser[c.UserID] = make(map[int64]bool)
		}
		doneByUser[c.UserID][c.TaskID] = true
	}

	statuses := make([]UserStatus, 0, len(users))
	for _, u := range users {
		done := doneByUser[u.ID]
		if done == nil {
			done = map[int64]bool{}
		}
		statuses = append(statuses, UserStatus{User: u, Done: done})
	}
	return statuses, nil
}
