package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/ikkairat/telegram-checklist-bot/internal/bot"
	"github.com/ikkairat/telegram-checklist-bot/internal/config"
)

// Scheduler drives the bot's four recurring jobs from architecture doc §6:
// post the daily checklist, DM reminders before the deadline, close the
// day at the deadline, and compute the weekly penalty report.
type Scheduler struct {
	cron *cron.Cron
	cfg  *config.Config
	bot  *bot.Bot
}

func New(cfg *config.Config, b *bot.Bot) (*Scheduler, error) {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone %q: %w", cfg.Timezone, err)
	}

	s := &Scheduler{
		cron: cron.New(cron.WithLocation(loc)),
		cfg:  cfg,
		bot:  b,
	}
	if err := s.registerJobs(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Scheduler) registerJobs() error {
	pollSpec, err := dailySpec(s.cfg.DailyPollTime)
	if err != nil {
		return fmt.Errorf("daily_poll_time: %w", err)
	}
	if _, err := s.cron.AddFunc(pollSpec, s.job("post checklist", s.bot.PostChecklist)); err != nil {
		return fmt.Errorf("schedule daily poll: %w", err)
	}

	reminderTime, err := subtractMinutes(s.cfg.DayDeadlineTime, s.cfg.ReminderBeforeDeadlineMinutes)
	if err != nil {
		return fmt.Errorf("compute reminder time: %w", err)
	}
	reminderSpec, err := dailySpec(reminderTime)
	if err != nil {
		return fmt.Errorf("reminder time: %w", err)
	}
	if _, err := s.cron.AddFunc(reminderSpec, s.job("send reminders", s.bot.SendReminders)); err != nil {
		return fmt.Errorf("schedule reminders: %w", err)
	}

	deadlineSpec, err := dailySpec(s.cfg.DayDeadlineTime)
	if err != nil {
		return fmt.Errorf("day_deadline_time: %w", err)
	}
	if _, err := s.cron.AddFunc(deadlineSpec, s.job("close day", s.bot.CloseDay)); err != nil {
		return fmt.Errorf("schedule day close: %w", err)
	}

	reportWeekday, err := s.cfg.WeeklyReportWeekday()
	if err != nil {
		return err
	}
	weeklySpec, err := weeklySpec(s.cfg.WeeklyReportTime, reportWeekday)
	if err != nil {
		return fmt.Errorf("weekly_report_time: %w", err)
	}
	if _, err := s.cron.AddFunc(weeklySpec, s.job("weekly report", s.bot.PostWeeklyReport)); err != nil {
		return fmt.Errorf("schedule weekly report: %w", err)
	}

	return nil
}

// job wraps an action so every cron firing is logged uniformly, whether it
// succeeds, fails, or just reports "nothing to do".
func (s *Scheduler) job(name string, action func(context.Context) (string, error)) func() {
	return func() {
		result, err := action(context.Background())
		if err != nil {
			slog.Error("scheduled job failed", "job", name, "error", err)
			return
		}
		slog.Info("scheduled job ran", "job", name, "result", result)
	}
}

func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop waits for any in-flight job to finish before returning.
func (s *Scheduler) Stop() {
	<-s.cron.Stop().Done()
}
