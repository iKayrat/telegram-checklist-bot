package scheduler

import (
	"fmt"
	"time"
)

func parseHHMM(hhmm string) (hour, minute int, err error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid time %q, expected HH:MM: %w", hhmm, err)
	}
	return t.Hour(), t.Minute(), nil
}

// dailySpec returns a standard 5-field cron spec that fires once a day at hhmm.
func dailySpec(hhmm string) (string, error) {
	hour, minute, err := parseHHMM(hhmm)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d %d * * *", minute, hour), nil
}

// weeklySpec returns a standard 5-field cron spec that fires once a week at
// hhmm on weekday (cron's day-of-week field matches time.Weekday: 0=Sunday).
func weeklySpec(hhmm string, weekday time.Weekday) (string, error) {
	hour, minute, err := parseHHMM(hhmm)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d %d * * %d", minute, hour, int(weekday)), nil
}

// subtractMinutes returns the HH:MM time that is `minutes` before hhmm,
// wrapping across midnight if needed.
func subtractMinutes(hhmm string, minutes int) (string, error) {
	hour, minute, err := parseHHMM(hhmm)
	if err != nil {
		return "", err
	}
	base := time.Date(2000, 1, 1, hour, minute, 0, 0, time.UTC)
	shifted := base.Add(-time.Duration(minutes) * time.Minute)
	return shifted.Format("15:04"), nil
}
