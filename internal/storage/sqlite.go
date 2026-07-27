package storage

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	sqlitemigrate "github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// Open opens the SQLite database at dbPath and applies all pending
// migrations found in migrationsDir before returning the connection.
func Open(dbPath, migrationsDir string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	// WAL + busy_timeout: the bot's own update loop is single-threaded, but
	// the scheduler (internal/scheduler) runs cron jobs on separate
	// goroutines that can occasionally hit the DB at the same moment as a
	// live user interaction. Without these, a second concurrent writer gets
	// an immediate SQLITE_BUSY instead of waiting briefly — worth having
	// regardless, but the slower an SD card gets, the wider that window is.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set journal_mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set synchronous: %w", err)
	}

	if err := runMigrations(db, migrationsDir); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func runMigrations(db *sqlx.DB, migrationsDir string) error {
	driver, err := sqlitemigrate.WithInstance(db.DB, &sqlitemigrate.Config{})
	if err != nil {
		return fmt.Errorf("init migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+migrationsDir, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
