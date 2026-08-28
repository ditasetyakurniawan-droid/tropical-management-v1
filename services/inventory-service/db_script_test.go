package main

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

type testDBStep struct {
	op           string
	match        string
	columns      []string
	rows         [][]driver.Value
	lastInsertID int64
	rowsAffected int64
	err          error
}

type testDBScript struct {
	mu    sync.Mutex
	steps []testDBStep
}

func (s *testDBScript) take(op, query string) (testDBStep, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.steps) == 0 {
		return testDBStep{}, fmt.Errorf("unexpected database %s: %s", op, compactSQL(query))
	}
	step := s.steps[0]
	s.steps = s.steps[1:]
	if step.op != op {
		return testDBStep{}, fmt.Errorf("database operation mismatch: got %s want %s", op, step.op)
	}
	if step.match != "" && !strings.Contains(compactSQL(query), compactSQL(step.match)) {
		return testDBStep{}, fmt.Errorf("database query mismatch: got %q want substring %q", compactSQL(query), compactSQL(step.match))
	}
	return step, nil
}

func (s *testDBScript) assertDone(t *testing.T) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.steps) != 0 {
		t.Fatalf("%d scripted database operation(s) were not consumed", len(s.steps))
	}
}

func compactSQL(value string) string { return strings.Join(strings.Fields(value), " ") }

type testDBDriver struct{ script *testDBScript }
type testDBConn struct{ script *testDBScript }
type testDBTx struct{}
type testDBRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}
type testDBResult struct{ id, affected int64 }

func (d *testDBDriver) Open(string) (driver.Conn, error)  { return &testDBConn{script: d.script}, nil }
func (c *testDBConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c *testDBConn) Close() error                        { return nil }
func (c *testDBConn) Begin() (driver.Tx, error)           { return testDBTx{}, nil }
func (c *testDBConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return testDBTx{}, nil
}
func (c *testDBConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	step, err := c.script.take("exec", query)
	if err != nil {
		return nil, err
	}
	if step.err != nil {
		return nil, step.err
	}
	return testDBResult{id: step.lastInsertID, affected: step.rowsAffected}, nil
}
func (c *testDBConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	step, err := c.script.take("query", query)
	if err != nil {
		return nil, err
	}
	if step.err != nil {
		return nil, step.err
	}
	return &testDBRows{columns: step.columns, rows: step.rows}, nil
}
func (testDBTx) Commit() error          { return nil }
func (testDBTx) Rollback() error        { return nil }
func (r *testDBRows) Columns() []string { return r.columns }
func (r *testDBRows) Close() error      { return nil }
func (r *testDBRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}
func (r testDBResult) LastInsertId() (int64, error) { return r.id, nil }
func (r testDBResult) RowsAffected() (int64, error) { return r.affected, nil }

var testDBSequence uint64

func openTestDB(t *testing.T, steps ...testDBStep) (*sql.DB, *testDBScript) {
	t.Helper()
	script := &testDBScript{steps: append([]testDBStep(nil), steps...)}
	name := fmt.Sprintf("%s-testdb-%d", serviceName, atomic.AddUint64(&testDBSequence, 1))
	sql.Register(name, &testDBDriver{script: script})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, script
}

func row(values ...any) []driver.Value {
	out := make([]driver.Value, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

func queryStep(match string, columns []string, rows ...[]driver.Value) testDBStep {
	return testDBStep{op: "query", match: match, columns: columns, rows: rows}
}

func queryErrorStep(match string, err error) testDBStep {
	return testDBStep{op: "query", match: match, err: err}
}

func execStep(match string, lastInsertID, rowsAffected int64) testDBStep {
	return testDBStep{op: "exec", match: match, lastInsertID: lastInsertID, rowsAffected: rowsAffected}
}

func execErrorStep(match string, err error) testDBStep {
	return testDBStep{op: "exec", match: match, err: err}
}
