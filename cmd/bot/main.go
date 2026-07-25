package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ikkairat/telegram-checklist-bot/internal/bot"
	"github.com/ikkairat/telegram-checklist-bot/internal/config"
	"github.com/ikkairat/telegram-checklist-bot/internal/scheduler"
	"github.com/ikkairat/telegram-checklist-bot/internal/service"
	"github.com/ikkairat/telegram-checklist-bot/internal/storage"
)

func main() {
	configPath := flag.String("config", "config.json", "path to config.json")
	migrationsDir := flag.String("migrations", "migrations", "path to the migrations directory")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		logger.Error("create db directory", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.ReportsDir, 0o755); err != nil {
		logger.Error("create reports directory", "error", err)
		os.Exit(1)
	}

	db, err := storage.Open(cfg.DBPath, *migrationsDir)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	userRepo := storage.NewUserRepo(db)
	taskRepo := storage.NewTaskRepo(db)
	checkinRepo := storage.NewCheckinRepo(db)

	svc := service.NewChecklistService(
		userRepo,
		taskRepo,
		storage.NewDailyPollRepo(db),
		checkinRepo,
	)

	reportWeekday, err := cfg.WeeklyReportWeekday()
	if err != nil {
		logger.Error("parse weekly_report_day", "error", err)
		os.Exit(1)
	}
	penaltySvc := service.NewPenaltyService(
		userRepo,
		taskRepo,
		checkinRepo,
		storage.NewWeekRepo(db),
		storage.NewPenaltyRepo(db),
		storage.NewFundLedgerRepo(db),
		reportWeekday,
	)

	b, err := bot.New(cfg, svc, penaltySvc)
	if err != nil {
		logger.Error("init bot", "error", err)
		os.Exit(1)
	}

	sched, err := scheduler.New(cfg, b)
	if err != nil {
		logger.Error("init scheduler", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		logger.Info("shutting down")
		sched.Stop()
		b.Stop()
	}()

	sched.Start()
	logger.Info("bot starting")
	b.Start()
}
