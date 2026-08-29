package trafficx

import (
	"testing"
	"time"
)

func TestConcurrencyLimiter(t *testing.T) {
	l := NewConcurrencyLimiter(1)
	if !l.TryAcquire() {
		t.Fatal("first acquire should succeed")
	}
	if l.TryAcquire() {
		t.Fatal("second acquire should fail while saturated")
	}
	l.Release()
	if !l.TryAcquire() {
		t.Fatal("acquire should succeed after release")
	}
	l.Release()
}

func TestTokenBucketLimiterRefills(t *testing.T) {
	l := NewTokenBucketLimiter(2, time.Minute, 10)
	now := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)

	if ok, _ := l.Allow("alice", now); !ok {
		t.Fatal("first token should be allowed")
	}
	if ok, _ := l.Allow("alice", now); !ok {
		t.Fatal("second token should be allowed")
	}
	if ok, retry := l.Allow("alice", now); ok || retry <= 0 {
		t.Fatalf("third token should be limited, retry=%s", retry)
	}
	if ok, _ := l.Allow("alice", now.Add(30*time.Second)); !ok {
		t.Fatal("one token should refill after half the window")
	}
}

func TestTokenBucketLimiterBoundsKeys(t *testing.T) {
	l := NewTokenBucketLimiter(1, time.Minute, 1)
	now := time.Now()
	if ok, _ := l.Allow("alice", now); !ok {
		t.Fatal("alice should be allowed")
	}
	if ok, _ := l.Allow("bob", now); ok {
		t.Fatal("new key should be rejected while key capacity is occupied")
	}
	if ok, _ := l.Allow("bob", now.Add(2*time.Minute)); !ok {
		t.Fatal("idle key should be pruned after refill")
	}
}

func TestConnectionLimiter(t *testing.T) {
	l := NewConnectionLimiter(2, 1)
	if !l.TryAcquire("alice") {
		t.Fatal("alice should acquire")
	}
	if l.TryAcquire("alice") {
		t.Fatal("alice should hit per-key limit")
	}
	if !l.TryAcquire("bob") {
		t.Fatal("bob should acquire second global slot")
	}
	if l.TryAcquire("carol") {
		t.Fatal("carol should hit global limit")
	}
	if got := l.Active(); got != 2 {
		t.Fatalf("active=%d want 2", got)
	}
	l.Release("alice")
	l.Release("bob")
	if got := l.Active(); got != 0 {
		t.Fatalf("active=%d want 0", got)
	}
}
