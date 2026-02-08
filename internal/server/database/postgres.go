package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// migrations contains all database migrations in order.
// Each migration has a version key and SQL to execute.
// The SQL files in migrations/ at the project root mirror these for reference.
var migrations = []struct {
	Version string
	SQL     string
}{
	{
		Version: "000001_create_uploads",
		SQL: `
			CREATE TABLE IF NOT EXISTS uploads (
				id              VARCHAR(24)  PRIMARY KEY,
				filename        VARCHAR(255) NOT NULL,
				original_size   BIGINT       NOT NULL,
				compressed_size BIGINT       NOT NULL,
				file_hash       VARCHAR(64)  NOT NULL,
				uploaded_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
				expires_at      TIMESTAMPTZ  NOT NULL,
				download_count  INTEGER      NOT NULL DEFAULT 0,
				password_hash   VARCHAR(255),
				deletion_token  VARCHAR(48)  NOT NULL,
				created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
			);
			CREATE INDEX IF NOT EXISTS idx_uploads_expires_at ON uploads(expires_at);
			CREATE INDEX IF NOT EXISTS idx_uploads_file_hash ON uploads(file_hash);
		`,
	},
}

// DB wraps a pgxpool connection pool and provides health checks and migrations.
type DB struct {
	Pool *pgxpool.Pool
}

// New creates a new database connection pool.
func New(ctx context.Context, databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	slog.Info("connected to database")
	return &DB{Pool: pool}, nil
}

// RunMigrations applies all pending database migrations in order.
func (db *DB) RunMigrations(ctx context.Context) error {
	// Create migrations tracking table
	_, err := db.Pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	for _, m := range migrations {
		// Check if already applied
		var exists bool
		err := db.Pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)",
			m.Version,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", m.Version, err)
		}
		if exists {
			continue
		}

		// Execute migration in a transaction
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %s: %w", m.Version, err)
		}

		if _, err := tx.Exec(ctx, m.SQL); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to execute migration %s: %w", m.Version, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", m.Version); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to record migration %s: %w", m.Version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", m.Version, err)
		}

		slog.Info("applied migration", "version", m.Version)
	}

	return nil
}

// HealthCheck verifies the database connection is alive.
func (db *DB) HealthCheck(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

// Close shuts down the connection pool.
func (db *DB) Close() {
	db.Pool.Close()
}
