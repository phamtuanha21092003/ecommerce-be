package testutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/phamtuanha21092003/ecommerce-be/platform/database"
)

// Open connects to TEST_POSTGRESQL_URI and applies the catalog migrations.
// The test is skipped when the variable is unset.
func Open(t *testing.T) *database.SqlxDatabase {
	t.Helper()

	uri := os.Getenv("TEST_POSTGRESQL_URI")
	if uri == "" {
		t.Skip("TEST_POSTGRESQL_URI is not set")
	}

	if err := migrate(t, uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	db, err := database.OpenSQLX("postgres", uri)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// Reset removes store and product rows so each test starts from an empty catalog.
func Reset(t *testing.T, db *database.SqlxDatabase) {
	t.Helper()
	if _, err := db.Exec(`TRUNCATE TABLE catalog.products, platform.stores CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func migrate(t *testing.T, uri string) error {
	t.Helper()

	ctx := context.Background()
	cfg, err := pgx.ParseConfig(uri)
	if err != nil {
		return err
	}
	cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	root := moduleRoot(t)
	hasStores, err := exists(ctx, conn, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'platform' AND table_name = 'stores'
		)`)
	if err != nil {
		return err
	}
	if !hasStores {
		if err := execFile(ctx, conn, filepath.Join(root, "migrations", "000001_init_schema.up.sql")); err != nil {
			return err
		}
	}

	hasVersion, err := exists(ctx, conn, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'catalog' AND table_name = 'products' AND column_name = 'version'
		)`)
	if err != nil {
		return err
	}
	if !hasVersion {
		if err := execFile(ctx, conn, filepath.Join(root, "migrations", "000002_product_v1.up.sql")); err != nil {
			return err
		}
	}

	hasUploads, err := exists(ctx, conn, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'storage' AND table_name = 'upload_sessions'
		)`)
	if err != nil {
		return err
	}
	if !hasUploads {
		return execFile(ctx, conn, filepath.Join(root, "migrations", "000003_upload_sessions.up.sql"))
	}
	return nil
}

func exists(ctx context.Context, conn *pgx.Conn, query string) (bool, error) {
	var found bool
	err := conn.QueryRow(ctx, query).Scan(&found)
	return found, err
}

func execFile(ctx context.Context, conn *pgx.Conn, path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = conn.Exec(ctx, string(body))
	return err
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
