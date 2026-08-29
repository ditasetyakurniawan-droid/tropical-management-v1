package migrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// migrationSQL contains the versioned database schema for every service.
// Each file must contain exactly one SQL statement. MySQL DDL auto-commits,
// therefore a migration is marked dirty before execution and clean afterwards.
//
//go:embed sql/*/*.sql
var migrationSQL embed.FS

const createMigrationsTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    checksum CHAR(64) NOT NULL,
    dirty BOOLEAN NOT NULL DEFAULT TRUE,
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    applied_at TIMESTAMP NULL DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

type Migration struct {
	Version  int64
	Name     string
	SQL      string
	Checksum string
}

type Target struct {
	Name      string
	DB        *sql.DB
	VerifySQL []string
}

func Load(service string) ([]Migration, error) {
	service = strings.TrimSpace(service)
	if service == "" || strings.Contains(service, "/") || strings.Contains(service, "\\") {
		return nil, fmt.Errorf("invalid migration service %q", service)
	}

	paths, err := fs.Glob(migrationSQL, "sql/"+service+"/*.sql")
	if err != nil {
		return nil, fmt.Errorf("glob migrations for %s: %w", service, err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no migrations found for %s", service)
	}
	sort.Strings(paths)

	out := make([]Migration, 0, len(paths))
	var previous int64
	for _, path := range paths {
		migration, err := loadFile(path)
		if err != nil {
			return nil, err
		}
		if migration.Version <= previous {
			return nil, fmt.Errorf("migration versions for %s are not strictly increasing", service)
		}
		previous = migration.Version
		out = append(out, migration)
	}
	return out, nil
}

func loadFile(path string) (Migration, error) {
	base := filepath.Base(path)
	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 || !strings.HasSuffix(parts[1], ".sql") {
		return Migration{}, fmt.Errorf("invalid migration filename %q; expected NNNNNN_name.sql", base)
	}
	version, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || version <= 0 {
		return Migration{}, fmt.Errorf("invalid migration version in %q", base)
	}
	body, err := migrationSQL.ReadFile(path)
	if err != nil {
		return Migration{}, fmt.Errorf("read migration %q: %w", base, err)
	}
	statement := strings.TrimSpace(string(body))
	if statement == "" {
		return Migration{}, fmt.Errorf("migration %q is empty", base)
	}
	sum := sha256.Sum256([]byte(statement))
	return Migration{
		Version:  version,
		Name:     strings.TrimSuffix(parts[1], ".sql"),
		SQL:      statement,
		Checksum: hex.EncodeToString(sum[:]),
	}, nil
}

func Up(ctx context.Context, target Target) error {
	if target.DB == nil {
		return errors.New("migration database is nil")
	}
	migrations, err := Load(target.Name)
	if err != nil {
		return err
	}

	conn, err := target.DB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection for %s: %w", target.Name, err)
	}
	defer conn.Close()

	lockName := "tropical:migrate:" + target.Name
	if err := acquireLock(ctx, conn, lockName); err != nil {
		return err
	}
	defer releaseLock(conn, lockName)

	if _, err := conn.ExecContext(ctx, createMigrationsTable); err != nil {
		return fmt.Errorf("create schema_migrations for %s: %w", target.Name, err)
	}

	for _, migration := range migrations {
		if err := applyOne(ctx, conn, target.Name, migration); err != nil {
			return err
		}
	}
	for _, query := range target.VerifySQL {
		rows, err := conn.QueryContext(ctx, query)
		if err != nil {
			return fmt.Errorf("verify schema %s: %w", target.Name, err)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("close schema verification rows for %s: %w", target.Name, err)
		}
	}
	return nil
}

func acquireLock(ctx context.Context, conn *sql.Conn, name string) error {
	var acquired sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, ?)", name, 30).Scan(&acquired); err != nil {
		return fmt.Errorf("acquire migration lock %q: %w", name, err)
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		return fmt.Errorf("migration lock %q not acquired within 30s", name)
	}
	return nil
}

func releaseLock(conn *sql.Conn, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var ignored sql.NullInt64
	_ = conn.QueryRowContext(ctx, "SELECT RELEASE_LOCK(?)", name).Scan(&ignored)
}

func applyOne(ctx context.Context, conn *sql.Conn, service string, migration Migration) error {
	var (
		checksum string
		dirty    bool
	)
	err := conn.QueryRowContext(ctx,
		"SELECT checksum, dirty FROM schema_migrations WHERE version = ?",
		migration.Version,
	).Scan(&checksum, &dirty)

	switch {
	case err == nil:
		if dirty {
			return fmt.Errorf("%s migration %06d is dirty; inspect the database before retrying", service, migration.Version)
		}
		if checksum != migration.Checksum {
			return fmt.Errorf("%s migration %06d checksum changed after it was applied", service, migration.Version)
		}
		return nil
	case !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("read %s migration %06d state: %w", service, migration.Version, err)
	}

	if _, err := conn.ExecContext(ctx,
		"INSERT INTO schema_migrations(version,name,checksum,dirty) VALUES(?,?,?,TRUE)",
		migration.Version, migration.Name, migration.Checksum,
	); err != nil {
		return fmt.Errorf("mark %s migration %06d dirty: %w", service, migration.Version, err)
	}

	if _, err := conn.ExecContext(ctx, migration.SQL); err != nil {
		return fmt.Errorf("apply %s migration %06d_%s: %w", service, migration.Version, migration.Name, err)
	}

	if _, err := conn.ExecContext(ctx,
		"UPDATE schema_migrations SET dirty=FALSE, applied_at=CURRENT_TIMESTAMP WHERE version=?",
		migration.Version,
	); err != nil {
		return fmt.Errorf("mark %s migration %06d clean: %w", service, migration.Version, err)
	}
	return nil
}
