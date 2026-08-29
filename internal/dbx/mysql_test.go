package dbx

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

type fakePinger struct {
	errors []error
	calls  int
}

func (f *fakePinger) PingContext(context.Context) error {
	f.calls++
	if len(f.errors) == 0 {
		return nil
	}
	idx := f.calls - 1
	if idx >= len(f.errors) {
		idx = len(f.errors) - 1
	}
	return f.errors[idx]
}

func TestWaitForPingSucceedsAfterRetry(t *testing.T) {
	p := &fakePinger{errors: []error{errors.New("not ready"), errors.New("still starting"), nil}}
	var sleeps int
	err := waitForPing(p, 3, time.Millisecond, time.Second, func(time.Duration) { sleeps++ })
	if err != nil {
		t.Fatal(err)
	}
	if p.calls != 3 || sleeps != 2 {
		t.Fatalf("calls=%d sleeps=%d", p.calls, sleeps)
	}
}

func TestWaitForPingReturnsLastError(t *testing.T) {
	p := &fakePinger{errors: []error{errors.New("down")}}
	err := waitForPing(p, 2, 0, time.Second, nil)
	if err == nil || !strings.Contains(err.Error(), "after 2 attempts") || !strings.Contains(err.Error(), "down") {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.calls != 2 {
		t.Fatalf("calls=%d, want 2", p.calls)
	}
}

func TestWaitForPingRejectsInvalidConfiguration(t *testing.T) {
	if err := waitForPing(&fakePinger{}, 0, 0, time.Second, nil); err == nil {
		t.Fatal("expected invalid attempt count to fail")
	}
	if err := waitForPing(&fakePinger{}, 1, 0, 0, nil); err == nil {
		t.Fatal("expected invalid timeout to fail")
	}
}

func TestApplyNetworkTimeouts(t *testing.T) {
	cfg := RuntimeConfig()
	dsn, err := applyNetworkTimeouts("user:pass@tcp(mysql:3306)/db?parseTime=true", cfg)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Timeout != cfg.ConnectTimeout || parsed.ReadTimeout != cfg.ReadTimeout || parsed.WriteTimeout != cfg.WriteTimeout {
		t.Fatalf("network timeouts not applied: %+v", parsed)
	}
}

func TestRuntimeConfigRejectsIdleGreaterThanOpen(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "3")
	t.Setenv("DB_MAX_IDLE_CONNS", "4")
	defer func() {
		if recover() == nil {
			t.Fatal("expected invalid pool configuration to panic")
		}
	}()
	_ = RuntimeConfig()
}

func TestWithQueryTimeout(t *testing.T) {
	ctx, cancel := WithQueryTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 30*time.Millisecond {
		t.Fatal("expected bounded query context")
	}
}

func TestIsDuplicateKey(t *testing.T) {
	if !IsDuplicateKey(&mysql.MySQLError{Number: 1062, Message: "duplicate"}) {
		t.Fatal("expected MySQL 1062 to be recognized as duplicate key")
	}
	if IsDuplicateKey(&mysql.MySQLError{Number: 1045, Message: "access denied"}) {
		t.Fatal("non-duplicate MySQL error must not be recognized as duplicate key")
	}
	if IsDuplicateKey(errors.New("plain error")) {
		t.Fatal("plain error must not be recognized as duplicate key")
	}
}
