package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Config mirrors config.json — see config.example.json at the repo root.
type Config struct {
	BotToken                      string  `json:"bot_token"`
	GroupChatID                   int64   `json:"group_chat_id"`
	GroupTopicID                  int     `json:"group_topic_id"` // forum topic (message_thread_id) to post in; 0 = General/no topic
	AdminTelegramIDs              []int64 `json:"admin_telegram_ids"`
	Timezone                      string  `json:"timezone"`
	DailyPollTime                 string  `json:"daily_poll_time"`
	ReminderBeforeDeadlineMinutes int     `json:"reminder_before_deadline_minutes"`
	DayDeadlineTime               string  `json:"day_deadline_time"`
	WeeklyReportDay               string  `json:"weekly_report_day"`
	WeeklyReportTime              string  `json:"weekly_report_time"`
	DBPath                        string  `json:"db_path"`
	ReportsDir                    string  `json:"reports_dir"`
}

// Load reads and parses the config file at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if cfg.BotToken == "" {
		return nil, fmt.Errorf("config: bot_token is required")
	}
	if cfg.GroupChatID == 0 {
		return nil, fmt.Errorf("config: group_chat_id is required")
	}

	return &cfg, nil
}

// IsAdmin reports whether telegramID belongs to a configured admin.
func (c *Config) IsAdmin(telegramID int64) bool {
	for _, id := range c.AdminTelegramIDs {
		if id == telegramID {
			return true
		}
	}
	return false
}

var weekdaysByName = map[string]time.Weekday{
	"sunday":    time.Sunday,
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
}

// WeeklyReportWeekday parses WeeklyReportDay (e.g. "sunday") into a time.Weekday.
func (c *Config) WeeklyReportWeekday() (time.Weekday, error) {
	day, ok := weekdaysByName[strings.ToLower(strings.TrimSpace(c.WeeklyReportDay))]
	if !ok {
		return 0, fmt.Errorf("config: invalid weekly_report_day %q", c.WeeklyReportDay)
	}
	return day, nil
}
