package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ikkairat/telegram-checklist-bot/internal/domain"
	"github.com/ikkairat/telegram-checklist-bot/internal/storage"
)

// TestUserWeekStatusOnlyCountsElapsedDays reproduces a real /report output:
// mid-week, with only 2 of 7 days elapsed, the not-yet-happened remaining
// days must not be counted as "missed".
func TestUserWeekStatusOnlyCountsElapsedDays(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.Open(dbPath, "../../migrations")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	userRepo := storage.NewUserRepo(db)
	taskRepo := storage.NewTaskRepo(db)
	checkinRepo := storage.NewCheckinRepo(db)
	weekRepo := storage.NewWeekRepo(db)

	if err := userRepo.Upsert(ctx, &domain.User{TelegramID: 1, FullName: "Kairat"}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	user, err := userRepo.GetByTelegramID(ctx, 1)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	task, err := taskRepo.Create(ctx, "Спорт", 0)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// "Today" is the 3rd day (index 2) of a Thu-Wed week — 2 days have
	// already elapsed (day 0 and day 1), 5 days are still ahead.
	weekStart := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC) // Friday
	today := weekStart.AddDate(0, 0, 1)                       // Saturday — 2nd elapsed day
	weekEnd := weekStart.AddDate(0, 0, 6)

	if _, err := weekRepo.Create(ctx, weekStart, weekEnd, 100); err != nil {
		t.Fatalf("create week: %v", err)
	}

	// Completed the task on both elapsed days.
	for _, d := range []time.Time{weekStart, today} {
		if _, err := checkinRepo.Toggle(ctx, user.ID, task.ID, d); err != nil {
			t.Fatalf("toggle checkin for %v: %v", d, err)
		}
	}

	svc := NewPenaltyService(userRepo, taskRepo, checkinRepo, weekRepo, storage.NewPenaltyRepo(db), storage.NewFundLedgerRepo(db), time.Wednesday)

	start, end, total, completed, missed, err := svc.UserWeekStatus(ctx, today, user.ID)
	if err != nil {
		t.Fatalf("UserWeekStatus: %v", err)
	}

	if !start.Equal(weekStart) || !end.Equal(weekEnd) {
		t.Fatalf("expected week range %v..%v, got %v..%v", weekStart, weekEnd, start, end)
	}
	if total != 2 {
		t.Fatalf("expected totalTasks=2 (1 task x 2 elapsed days), got %d", total)
	}
	if completed != 2 {
		t.Fatalf("expected completed=2, got %d", completed)
	}
	if missed != 0 {
		t.Fatalf("expected missed=0 (not-yet-happened days must not count as missed), got %d", missed)
	}
}
