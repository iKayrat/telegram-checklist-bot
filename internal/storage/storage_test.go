package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/ikkairat/telegram-checklist-bot/internal/domain"
)

func openTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(dbPath, "../../migrations")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenRunsMigrations(t *testing.T) {
	db := openTestDB(t)
	var count int
	if err := db.Get(&count, `SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'users'`); err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected users table to exist, got count=%d", count)
	}
}

func TestUserRepoUpsertAndGet(t *testing.T) {
	db := openTestDB(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	u := &domain.User{TelegramID: 42, Username: "neo", FullName: "Neo Anderson"}
	if err := repo.Upsert(ctx, u); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := repo.GetByTelegramID(ctx, 42)
	if err != nil {
		t.Fatalf("get by telegram id: %v", err)
	}
	if got.FullName != "Neo Anderson" {
		t.Fatalf("expected full name %q, got %q", "Neo Anderson", got.FullName)
	}
	if !got.IsActive {
		t.Fatalf("expected new user to be active by default")
	}
}

func TestUserRepoUpsertReactivates(t *testing.T) {
	db := openTestDB(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	if err := repo.Upsert(ctx, &domain.User{TelegramID: 99, Username: "morpheus", FullName: "Morpheus"}); err != nil {
		t.Fatalf("initial upsert: %v", err)
	}
	if err := repo.SetActive(ctx, 99, false); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	got, err := repo.GetByTelegramID(ctx, 99)
	if err != nil {
		t.Fatalf("get by telegram id: %v", err)
	}
	if got.IsActive {
		t.Fatalf("expected user to be inactive after SetActive(false)")
	}

	// Re-running /start (Upsert) must reactivate the user.
	if err := repo.Upsert(ctx, &domain.User{TelegramID: 99, Username: "morpheus", FullName: "Morpheus"}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err = repo.GetByTelegramID(ctx, 99)
	if err != nil {
		t.Fatalf("get by telegram id: %v", err)
	}
	if !got.IsActive {
		t.Fatalf("expected upsert to reactivate the user")
	}
}

func TestUserRepoSetAdminAndGetByUsername(t *testing.T) {
	db := openTestDB(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	u := &domain.User{TelegramID: 7, Username: "trinity", FullName: "Trinity"}
	if err := repo.Upsert(ctx, u); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := repo.GetByUsername(ctx, "trinity")
	if err != nil {
		t.Fatalf("get by username: %v", err)
	}
	if got.IsAdmin {
		t.Fatalf("expected new user to not be admin")
	}

	if err := repo.SetAdmin(ctx, 7, true); err != nil {
		t.Fatalf("set admin: %v", err)
	}

	got, err = repo.GetByTelegramID(ctx, 7)
	if err != nil {
		t.Fatalf("get by telegram id: %v", err)
	}
	if !got.IsAdmin {
		t.Fatalf("expected user to be admin after SetAdmin(true)")
	}

	if err := repo.SetAdmin(ctx, 7, false); err != nil {
		t.Fatalf("unset admin: %v", err)
	}
	got, err = repo.GetByTelegramID(ctx, 7)
	if err != nil {
		t.Fatalf("get by telegram id: %v", err)
	}
	if got.IsAdmin {
		t.Fatalf("expected user to not be admin after SetAdmin(false)")
	}
}

func TestCheckinRepoToggle(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	userRepo := NewUserRepo(db)
	taskRepo := NewTaskRepo(db)
	checkinRepo := NewCheckinRepo(db)

	if err := userRepo.Upsert(ctx, &domain.User{TelegramID: 1, FullName: "Alice"}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	user, err := userRepo.GetByTelegramID(ctx, 1)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	task, err := taskRepo.Create(ctx, "Read a book", 1)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	date := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)

	checked, err := checkinRepo.Toggle(ctx, user.ID, task.ID, date)
	if err != nil {
		t.Fatalf("toggle: %v", err)
	}
	if !checked {
		t.Fatalf("expected first toggle to check the task")
	}

	checked, err = checkinRepo.Toggle(ctx, user.ID, task.ID, date)
	if err != nil {
		t.Fatalf("toggle again: %v", err)
	}
	if checked {
		t.Fatalf("expected second toggle to uncheck the task")
	}
}
