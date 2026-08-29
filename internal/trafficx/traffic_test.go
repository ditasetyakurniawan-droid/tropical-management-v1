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

func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

func TestLimiterConstructorValidation(t *testing.T) {
	mustPanic(t, func() { NewConcurrencyLimiter(0) })
	mustPanic(t, func() { NewTokenBucketLimiter(0, time.Minute, 1) })
	mustPanic(t, func() { NewTokenBucketLimiter(1, 0, 1) })
	mustPanic(t, func() { NewTokenBucketLimiter(1, time.Minute, 0) })
	mustPanic(t, func() { NewConnectionLimiter(0, 1) })
	mustPanic(t, func() { NewConnectionLimiter(1, 0) })
	mustPanic(t, func() { NewConnectionLimiter(1, 2) })
}

func TestNilLimitersAreNoOp(t *testing.T) {
	var concurrency *ConcurrencyLimiter
	if !concurrency.TryAcquire() {
		t.Fatal("nil concurrency limiter should allow")
	}
	concurrency.Release()

	var bucket *TokenBucketLimiter
	if ok, retry := bucket.Allow("", time.Now()); !ok || retry != 0 {
		t.Fatalf("nil token bucket returned ok=%v retry=%s", ok, retry)
	}

	var connections *ConnectionLimiter
	if !connections.TryAcquire("") {
		t.Fatal("nil connection limiter should allow")
	}
	connections.Release("")
	if got := connections.Active(); got != 0 {
		t.Fatalf("nil connection limiter Active()=%d", got)
	}
}

func TestConcurrencyLimiterRejectsUnbalancedRelease(t *testing.T) {
	mustPanic(t, func() { NewConcurrencyLimiter(1).Release() })
}

func TestTokenBucketHandlesBlankKeyClockSkewAndCapacityCap(t *testing.T) {
	l := NewTokenBucketLimiter(2, time.Minute, 2)
	now := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)

	if ok, _ := l.Allow("", now); !ok {
		t.Fatal("blank key should use the shared fallback key")
	}
	if ok, _ := l.Allow("", now); !ok {
		t.Fatal("second fallback-key token should be allowed")
	}
	if ok, retry := l.Allow("", now); ok || retry <= 0 {
		t.Fatalf("exhausted fallback key should be limited, retry=%s", retry)
	}

	// A backwards clock must not mint tokens.
	if ok, _ := l.Allow("", now.Add(-time.Second)); ok {
		t.Fatal("clock rollback should not refill the bucket")
	}

	// A long idle period refills only to capacity, not beyond it.
	later := now.Add(10 * time.Minute)
	if ok, _ := l.Allow("", later); !ok {
		t.Fatal("bucket should refill after a long idle period")
	}
	if ok, _ := l.Allow("", later); !ok {
		t.Fatal("refill should restore the full two-token capacity")
	}
	if ok, _ := l.Allow("", later); ok {
		t.Fatal("refill must be capped at configured capacity")
	}
}

func TestConnectionLimiterBlankKeyMultipleSlotsAndInvalidRelease(t *testing.T) {
	l := NewConnectionLimiter(3, 2)
	if !l.TryAcquire("") || !l.TryAcquire("") {
		t.Fatal("fallback key should acquire up to its per-key limit")
	}
	if l.TryAcquire("") {
		t.Fatal("fallback key should respect the per-key limit")
	}
	l.Release("")
	if got := l.Active(); got != 1 {
		t.Fatalf("active=%d want 1 after first release", got)
	}
	l.Release("")
	if got := l.Active(); got != 0 {
		t.Fatalf("active=%d want 0 after second release", got)
	}
	mustPanic(t, func() { l.Release("") })
}
