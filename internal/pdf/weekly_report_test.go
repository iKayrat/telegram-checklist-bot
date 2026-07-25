package pdf

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ikkairat/telegram-checklist-bot/internal/domain"
	"github.com/ikkairat/telegram-checklist-bot/internal/service"
)

func TestGenerateWeeklyReport(t *testing.T) {
	week := &domain.Week{
		ID:            1,
		StartDate:     time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
		EndDate:       time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC),
		PenaltyAmount: 100,
	}
	results := []service.UserPenalty{
		{
			User: domain.User{FullName: "Иван Иванов"},
			Penalty: domain.Penalty{
				TotalTasks: 14, MissedTasks: 2, Amount: 200,
			},
		},
		{
			User: domain.User{FullName: "Пётр Петров"},
			Penalty: domain.Penalty{
				TotalTasks: 14, MissedTasks: 0, Amount: 0, IsPaid: true,
			},
		},
	}

	path := filepath.Join(t.TempDir(), "report.pdf")
	if err := GenerateWeeklyReport(week, results, 200, 1200, path); err != nil {
		t.Fatalf("GenerateWeeklyReport: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat generated pdf: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("generated pdf is empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated pdf: %v", err)
	}
	if string(data[:4]) != "%PDF" {
		t.Fatalf("expected file to start with %%PDF header, got %q", data[:4])
	}
}
