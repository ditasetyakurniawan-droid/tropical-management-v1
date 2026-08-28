package dbx

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

const (
	defaultPingAttempts = 30
	defaultPingDelay    = 2 * time.Second
)

type pinger interface {
	Ping() error
}

// Open waits for MySQL because containers may start a few seconds before DB-dt/local MySQL is ready.
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	configurePool(db)

	if err := waitForPing(db, defaultPingAttempts, defaultPingDelay, time.Sleep); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func configurePool(db *sql.DB) {
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(3 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)
}

func waitForPing(db pinger, attempts int, delay time.Duration, sleep func(time.Duration)) error {
	if attempts < 1 {
		return fmt.Errorf("mysql ping attempts must be positive")
	}
	if sleep == nil {
		sleep = time.Sleep
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := db.Ping(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt < attempts && delay > 0 {
			sleep(delay)
		}
	}
	return fmt.Errorf("mysql unavailable after %d attempts: %w", attempts, lastErr)
}

// IsDuplicateKey reports whether err is a MySQL duplicate-entry error.
func IsDuplicateKey(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
