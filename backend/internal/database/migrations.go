package database

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Migrator struct{}

// RunMigrations executes pending SQL migrations in deterministic order with tracking and advisory locks.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// 1. Acquire PostgreSQL advisory lock to ensure safe concurrent startup
	const advisoryLockID = int64(839217482)
	if _, err := pool.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockID); err != nil {
		return fmt.Errorf("acquiring migration advisory lock: %w", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", advisoryLockID)
	}()

	// 2. Ensure schema_migrations table exists
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`
	if _, err := pool.Exec(ctx, createTableSQL); err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	// 3. Reconcile legacy database state (if schema_migrations is missing entries for existing tables)
	if err := reconcileLegacySchema(ctx, pool); err != nil {
		return fmt.Errorf("reconciling legacy schema: %w", err)
	}

	// 4. Read migration files from embedded filesystem
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading migrations dir: %w", err)
	}

	var filenames []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			filenames = append(filenames, e.Name())
		}
	}
	sort.Strings(filenames)

	// 5. Apply each pending migration inside its own transaction
	for _, filename := range filenames {
		var alreadyApplied bool
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", filename).Scan(&alreadyApplied)
		if err != nil {
			return fmt.Errorf("checking migration status for %s: %w", filename, err)
		}
		if alreadyApplied {
			log.Debug().Str("migration", filename).Msg("migration already applied, skipping")
			continue
		}

		sqlBytes, err := migrationsFS.ReadFile("migrations/" + filename)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", filename, err)
		}

		log.Info().Str("migration", filename).Msg("applying migration")
		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("beginning tx for %s: %w", filename, err)
		}

		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("executing migration %s: %w", filename, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, applied_at) VALUES ($1, now())", filename); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("recording migration %s: %w", filename, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("committing migration %s: %w", filename, err)
		}
		log.Info().Str("migration", filename).Msg("migration applied successfully")
	}

	return nil
}

// reconcileLegacySchema detects if historical tables exist from previous runs without schema_migrations tracking,
// and safely records them only when their required schema signatures are verified.
func reconcileLegacySchema(ctx context.Context, pool *pgxpool.Pool) error {
	type legacyCheck struct {
		version string
		table   string
		column  string
	}

	checks := []legacyCheck{
		{version: "001_create_users.sql", table: "users", column: "email"},
		{version: "002_create_intentions.sql", table: "intentions", column: "title"},
		{version: "003_create_tasks.sql", table: "tasks", column: "status"},
		{version: "004_create_task_dependencies.sql", table: "task_dependencies", column: "depends_on_task_id"},
		{version: "005_create_user_availability.sql", table: "user_availability", column: "start_time"},
		{version: "006_create_schedule_blocks.sql", table: "schedule_blocks", column: "start_timestamp"},
		{version: "007_create_plan_revisions.sql", table: "plan_revisions", column: "description"},
	}

	for _, c := range checks {
		var hasTable bool
		query := `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2
			);
		`
		if err := pool.QueryRow(ctx, query, c.table, c.column).Scan(&hasTable); err != nil {
			return fmt.Errorf("verifying table %s column %s: %w", c.table, c.column, err)
		}

		if hasTable {
			// Record as applied if not already in schema_migrations
			_, err := pool.Exec(ctx, `
				INSERT INTO schema_migrations (version, applied_at) 
				VALUES ($1, now()) 
				ON CONFLICT (version) DO NOTHING
			`, c.version)
			if err != nil {
				return fmt.Errorf("baselining legacy migration %s: %w", c.version, err)
			}
			log.Info().Str("migration", c.version).Msg("reconciled existing database table into schema_migrations")
		}
	}

	return nil
}
