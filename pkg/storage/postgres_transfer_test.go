package storage_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/model"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
	_ "github.com/lib/pq"
)

func TestPostgresImportsSQLiteOnBootstrap(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" { t.Skip("TEST_POSTGRES_URL is not configured") }
	admin, err := sql.Open("postgres", url)
	if err != nil { t.Fatalf("open PostgreSQL: %v", err) }
	t.Cleanup(func() { _ = admin.Close() })
	if _, err = admin.Exec(`DROP SCHEMA public CASCADE; CREATE SCHEMA public`); err != nil { t.Fatalf("reset PostgreSQL test schema: %v", err) }

	path := filepath.Join(t.TempDir(), "source.db")
	source, err := storage.New(storage.Options{SQLitePath: path})
	if err != nil { t.Fatalf("open SQLite source: %v", err) }
	ctx := context.Background()
	if _, err = source.UpsertUser(ctx, "Transfer@Example.test", "SQLite Transfer", ""); err != nil { t.Fatal(err) }
	presetID, err := source.CreateLabelLayoutPreset(ctx, model.LabelLayoutPreset{Name: "Transfer preset", LabelLayout: "[]"})
	if err != nil { t.Fatal(err) }
	if presetID == 0 { t.Fatal("SQLite preset did not receive an ID") }
	if err = source.Close(); err != nil { t.Fatal(err) }

	target, err := storage.New(storage.Options{SQLitePath: path, DatabaseURL: url})
	if err != nil { t.Fatalf("initialize PostgreSQL transfer: %v", err) }
	t.Cleanup(func() { _ = target.Close() })
	user, err := target.GetUserByEmail(ctx, "transfer@example.test")
	if err != nil || user == nil || user.Name != "SQLite Transfer" { t.Fatalf("transferred user = %#v, %v", user, err) }
	presets, err := target.GetLabelLayoutPresets(ctx)
	if err != nil || len(presets) != 1 || presets[0].Name != "Transfer preset" { t.Fatalf("transferred presets = %#v, %v", presets, err) }

	if _, err = os.Stat(path); err != nil { t.Fatalf("source SQLite was modified or removed: %v", err) }
}
