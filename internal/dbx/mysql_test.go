package dbx

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type fakePinger struct {
	errors []error
	calls  int
}

func (f *fakePinger) Ping() error {
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
	err := waitForPing(p, 3, time.Millisecond, func(time.Duration) { sleeps++ })
	if err != nil {
		t.Fatal(err)
	}
	if p.calls != 3 || sleeps != 2 {
		t.Fatalf("calls=%d sleeps=%d", p.calls, sleeps)
	}
}

func TestWaitForPingReturnsLastError(t *testing.T) {
	p := &fakePinger{errors: []error{errors.New("down")}}
	err := waitForPing(p, 2, 0, nil)
	if err == nil || !strings.Contains(err.Error(), "after 2 attempts") || !strings.Contains(err.Error(), "down") {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.calls != 2 {
		t.Fatalf("calls=%d, want 2", p.calls)
	}
}

func TestWaitForPingRejectsInvalidAttempts(t *testing.T) {
	if err := waitForPing(&fakePinger{}, 0, 0, nil); err == nil {
		t.Fatal("expected invalid attempt count to fail")
	}
}
