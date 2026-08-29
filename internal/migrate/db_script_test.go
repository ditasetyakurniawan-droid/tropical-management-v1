package migrate

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type dbStep struct {
	op      string
	match   string
	columns []string
	rows    [][]driver.Value
	err     error
}

type dbScript struct {
	mu    sync.Mutex
	steps []dbStep
}

func (s *dbScript) take(op, query string) (dbStep, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.steps) == 0 {
		return dbStep{}, fmt.Errorf("unexpected database %s: %s", op, compact(query))
	}
	step := s.steps[0]
	s.steps = s.steps[1:]
	if step.op != op {
		return dbStep{}, fmt.Errorf("operation mismatch: got %s want %s", op, step.op)
	}
	if step.match != "" && !strings.Contains(compact(query), compact(step.match)) {
		return dbStep{}, fmt.Errorf("query mismatch: got %q want %q", compact(query), compact(step.match))
	}
	return step, nil
}

func (s *dbScript) done(t *testing.T) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.steps) != 0 {
		t.Fatalf("%d database step(s) not consumed", len(s.steps))
	}
}

func compact(s string) string { return strings.Join(strings.Fields(s), " ") }

type dbDriver struct{ script *dbScript }
type dbConn struct{ script *dbScript }
type dbRows struct {
	columns []string
	rows    [][]driver.Value
	pos     int
}
type dbResult struct{}

func (d *dbDriver) Open(string) (driver.Conn, error)  { return &dbConn{script: d.script}, nil }
func (c *dbConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c *dbConn) Close() error                        { return nil }
func (c *dbConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }
func (c *dbConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	step, err := c.script.take("exec", query)
	if err != nil {
		return nil, err
	}
	if step.err != nil {
		return nil, step.err
	}
	return dbResult{}, nil
}
func (c *dbConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	step, err := c.script.take("query", query)
	if err != nil {
		return nil, err
	}
	if step.err != nil {
		return nil, step.err
	}
	return &dbRows{columns: step.columns, rows: step.rows}, nil
}
func (r *dbRows) Columns() []string { return r.columns }
func (r *dbRows) Close() error      { return nil }
func (r *dbRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.pos])
	r.pos++
	return nil
}
func (dbResult) LastInsertId() (int64, error) { return 0, nil }
func (dbResult) RowsAffected() (int64, error) { return 1, nil }

var dbSeq uint64

func testDB(t *testing.T, steps ...dbStep) (*sql.DB, *dbScript) {
	t.Helper()
	script := &dbScript{steps: append([]dbStep(nil), steps...)}
	name := fmt.Sprintf("migrate-test-%d", atomic.AddUint64(&dbSeq, 1))
	sql.Register(name, &dbDriver{script: script})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, script
}

func q(match string, columns []string, rows ...[]driver.Value) dbStep {
	return dbStep{op: "query", match: match, columns: columns, rows: rows}
}
func e(match string) dbStep { return dbStep{op: "exec", match: match} }
func values(v ...any) []driver.Value {
	out := make([]driver.Value, len(v))
	for i := range v {
		out[i] = v[i]
	}
	return out
}
