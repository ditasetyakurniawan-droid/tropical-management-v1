package dbx

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Open waits for MySQL because containers may start a few seconds before DB-dt/local MySQL is ready.
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(3 * time.Minute)

	var lastErr error
	for attempt := 1; attempt <= 30; attempt++ {
		if err := db.Ping(); err == nil {
			return db, nil
		} else {
			lastErr = err
		}
		time.Sleep(2 * time.Second)
	}
	_ = db.Close()
	return nil, fmt.Errorf("mysql unavailable after retries: %w", lastErr)
}
