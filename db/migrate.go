package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Migrate(databaseURL string) error { return run(databaseURL, true) }

func Down(databaseURL string) error { return run(databaseURL, false) }

func run(databaseURL string, up bool) error {
	source, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("migration db: %w", err)
	}
	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("migration driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("migration: %w", err)
	}
	var step error
	if up {
		step = m.Up()
	} else {
		step = m.Down()
	}
	sourceErr, dbErr := m.Close()
	if errors.Is(step, migrate.ErrNoChange) {
		step = nil
	}
	if step != nil {
		return step
	}
	if sourceErr != nil {
		return sourceErr
	}
	return dbErr
}
