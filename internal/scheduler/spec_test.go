package scheduler

import (
	"testing"
	"time"
)

func TestDailySpec(t *testing.T) {
	spec, err := dailySpec("21:05")
	if err != nil {
		t.Fatalf("dailySpec: %v", err)
	}
	if spec != "5 21 * * *" {
		t.Fatalf("expected %q, got %q", "5 21 * * *", spec)
	}
}

func TestWeeklySpec(t *testing.T) {
	spec, err := weeklySpec("23:00", time.Sunday)
	if err != nil {
		t.Fatalf("weeklySpec: %v", err)
	}
	if spec != "0 23 * * 0" {
		t.Fatalf("expected %q, got %q", "0 23 * * 0", spec)
	}
}

func TestSubtractMinutesWithinSameHour(t *testing.T) {
	got, err := subtractMinutes("23:59", 60)
	if err != nil {
		t.Fatalf("subtractMinutes: %v", err)
	}
	if got != "22:59" {
		t.Fatalf("expected %q, got %q", "22:59", got)
	}
}

func TestSubtractMinutesAcrossMidnight(t *testing.T) {
	got, err := subtractMinutes("00:30", 60)
	if err != nil {
		t.Fatalf("subtractMinutes: %v", err)
	}
	if got != "23:30" {
		t.Fatalf("expected %q, got %q", "23:30", got)
	}
}

func TestParseHHMMInvalid(t *testing.T) {
	if _, _, err := parseHHMM("25:99"); err == nil {
		t.Fatal("expected error for invalid time")
	}
}
