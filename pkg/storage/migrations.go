package storage

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	sqlitedriver "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/sqlite/*.sql migrations/postgres/*.sql
var migrationFiles embed.FS

func runMigrations(db *sql.DB, dialect, dsn string) error {
	if dialect == "sqlite" {
		if err := adoptSQLiteLegacyColumns(db); err != nil {
			return err
		}
	}
	root := "migrations/" + dialect
	source, err := iofs.New(migrationFiles, root)
	if err != nil {
		return err
	}
	var driver database.Driver
	if dialect == "postgres" {
		driver, err = postgres.WithInstance(db, &postgres.Config{})
	} else {
		driver, err = sqlitedriver.WithInstance(db, &sqlitedriver.Config{})
	}
	if err != nil {
		_ = source.Close()
		return err
	}
	m, err := migrate.NewWithInstance("iofs", source, dialect, driver)
	if err != nil {
		_ = source.Close()
		return err
	}
	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func adoptSQLiteLegacyColumns(db *sql.DB) error {
	var exists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('users','page_designs')`).Scan(&exists); err != nil || exists == 0 {
		return err
	}
	for _, change := range []struct{ table, column, ddl string }{
		{"users", "locale", "ALTER TABLE users ADD COLUMN locale TEXT NOT NULL DEFAULT 'id'"},
		{"page_designs", "label_layout", "ALTER TABLE page_designs ADD COLUMN label_layout TEXT DEFAULT NULL"},
		{"page_designs", "settings_json", "ALTER TABLE page_designs ADD COLUMN settings_json TEXT NOT NULL DEFAULT '{}'"},
	} {
		rows, err := db.Query("PRAGMA table_info(" + change.table + ")")
		if err != nil {
			return err
		}
		found := false
		for rows.Next() {
			var cid int
			var name, typ string
			var notNull int
			var def any
			var pk int
			if err := rows.Scan(&cid, &name, &typ, &notNull, &def, &pk); err != nil {
				rows.Close()
				return err
			}
			if strings.EqualFold(name, change.column) {
				found = true
			}
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if !found {
			if _, err := db.Exec(change.ddl); err != nil {
				return fmt.Errorf("adopt legacy %s.%s: %w", change.table, change.column, err)
			}
		}
	}
	return nil
}

var _ fs.FS = migrationFiles
