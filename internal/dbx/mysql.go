package dbx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/configx"
	mysql "github.com/go-sql-driver/mysql"
)

const (
	defaultMaxOpenConns    = 10
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = 3 * time.Minute
	defaultConnMaxIdleTime = 1 * time.Minute
	defaultPingAttempts    = 30
	defaultPingDelay       = 2 * time.Second
	defaultPingTimeout     = 3 * time.Second
	defaultQueryTimeout    = 5 * time.Second
	defaultConnectTimeout  = 3 * time.Second
	defaultReadTimeout     = 5 * time.Second
	defaultWriteTimeout    = 5 * time.Second
)

// Config bounds database resource usage and failure duration. The same values
// apply to every DB-backed service unless overridden by environment variables.
type Config struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	PingAttempts    int
	PingDelay       time.Duration
	PingTimeout     time.Duration
	QueryTimeout    time.Duration
	ConnectTimeout  time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
}

// RuntimeConfig loads and validates database runtime settings. Invalid values
// fail fast so a pod never starts with an accidental unbounded configuration.
func RuntimeConfig() Config {
	cfg := Config{
		MaxOpenConns:    configx.Int("DB_MAX_OPEN_CONNS", defaultMaxOpenConns),
		MaxIdleConns:    configx.Int("DB_MAX_IDLE_CONNS", defaultMaxIdleConns),
		ConnMaxLifetime: configx.Duration("DB_CONN_MAX_LIFETIME", defaultConnMaxLifetime),
		ConnMaxIdleTime: configx.Duration("DB_CONN_MAX_IDLE_TIME", defaultConnMaxIdleTime),
		PingAttempts:    configx.Int("DB_PING_ATTEMPTS", defaultPingAttempts),
		PingDelay:       configx.Duration("DB_PING_DELAY", defaultPingDelay),
		PingTimeout:     configx.Duration("DB_PING_TIMEOUT", defaultPingTimeout),
		QueryTimeout:    configx.Duration("DB_QUERY_TIMEOUT", defaultQueryTimeout),
		ConnectTimeout:  configx.Duration("DB_CONNECT_TIMEOUT", defaultConnectTimeout),
		ReadTimeout:     configx.Duration("DB_READ_TIMEOUT", defaultReadTimeout),
		WriteTimeout:    configx.Duration("DB_WRITE_TIMEOUT", defaultWriteTimeout),
	}
	if cfg.MaxIdleConns > cfg.MaxOpenConns {
		panic(fmt.Sprintf("dbx: DB_MAX_IDLE_CONNS (%d) cannot exceed DB_MAX_OPEN_CONNS (%d)", cfg.MaxIdleConns, cfg.MaxOpenConns))
	}
	return cfg
}

type pinger interface {
	PingContext(context.Context) error
}

// Open loads RuntimeConfig and opens a bounded MySQL connection pool.
func Open(dsn string) (*sql.DB, error) {
	return OpenWithConfig(dsn, RuntimeConfig())
}

// OpenWithConfig opens MySQL with explicit network timeouts, pool bounds, and
// bounded startup retries. It is exported so services can reuse QueryTimeout
// from the exact same validated configuration.
func OpenWithConfig(dsn string, cfg Config) (*sql.DB, error) {
	boundedDSN, err := applyNetworkTimeouts(dsn, cfg)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("mysql", boundedDSN)
	if err != nil {
		return nil, err
	}
	configurePool(db, cfg)

	if err := waitForPing(db, cfg.PingAttempts, cfg.PingDelay, cfg.PingTimeout, time.Sleep); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func applyNetworkTimeouts(dsn string, cfg Config) (string, error) {
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "", fmt.Errorf("parse mysql dsn: %w", err)
	}
	if parsed.Timeout <= 0 {
		parsed.Timeout = cfg.ConnectTimeout
	}
	if parsed.ReadTimeout <= 0 {
		parsed.ReadTimeout = cfg.ReadTimeout
	}
	if parsed.WriteTimeout <= 0 {
		parsed.WriteTimeout = cfg.WriteTimeout
	}
	return parsed.FormatDSN(), nil
}

func configurePool(db *sql.DB, cfg Config) {
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
}

func waitForPing(p pinger, attempts int, delay, timeout time.Duration, sleep func(time.Duration)) error {
	if attempts < 1 {
		return fmt.Errorf("mysql ping attempts must be positive")
	}
	if timeout <= 0 {
		return fmt.Errorf("mysql ping timeout must be positive")
	}
	if sleep == nil {
		sleep = time.Sleep
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err := p.PingContext(ctx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < attempts && delay > 0 {
			sleep(delay)
		}
	}
	return fmt.Errorf("mysql unavailable after %d attempts: %w", attempts, lastErr)
}

// WithQueryTimeout creates a child context bounded by the validated database
// query timeout. Callers must always defer the returned cancel function.
func WithQueryTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if timeout <= 0 {
		timeout = defaultQueryTimeout
	}
	return context.WithTimeout(parent, timeout)
}

// IsDuplicateKey reports whether err is a MySQL duplicate-entry error.
func IsDuplicateKey(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
