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
