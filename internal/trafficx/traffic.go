package trafficx

import (
	"sync"
	"time"
)

// ConcurrencyLimiter bounds the number of in-flight operations. Acquisition is
// intentionally non-blocking so saturated callers fail fast instead of adding
// another queue inside the process.
type ConcurrencyLimiter struct {
	slots chan struct{}
}

func NewConcurrencyLimiter(max int) *ConcurrencyLimiter {
	if max <= 0 {
		panic("trafficx: concurrency limit must be positive")
	}
	return &ConcurrencyLimiter{slots: make(chan struct{}, max)}
}

func (l *ConcurrencyLimiter) TryAcquire() bool {
	if l == nil {
		return true
	}
	select {
	case l.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (l *ConcurrencyLimiter) Release() {
	if l == nil {
		return
	}
	select {
	case <-l.slots:
	default:
		panic("trafficx: release without acquire")
	}
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

// TokenBucketLimiter is a bounded, concurrency-safe keyed token-bucket limiter.
// Capacity defines the burst size, while window defines how long it takes to
// refill a completely empty bucket. maxKeys bounds memory used by attacker-
// controlled keys.
type TokenBucketLimiter struct {
	mu       sync.Mutex
	capacity float64
	window   time.Duration
	maxKeys  int
	buckets  map[string]tokenBucket
}

func NewTokenBucketLimiter(capacity int, window time.Duration, maxKeys int) *TokenBucketLimiter {
	if capacity <= 0 {
		panic("trafficx: token bucket capacity must be positive")
	}
	if window <= 0 {
		panic("trafficx: token bucket window must be positive")
	}
	if maxKeys <= 0 {
		panic("trafficx: token bucket max keys must be positive")
	}
	return &TokenBucketLimiter{
		capacity: float64(capacity),
		window:   window,
		maxKeys:  maxKeys,
		buckets:  make(map[string]tokenBucket),
	}
}

// Allow consumes one token for key. retryAfter is non-zero when the request is
// rejected and reports the earliest approximate time a token becomes available.
func (l *TokenBucketLimiter) Allow(key string, now time.Time) (allowed bool, retryAfter time.Duration) {
	if l == nil {
		return true, 0
	}
	if key == "" {
		key = "_"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= l.maxKeys {
			l.pruneIdleLocked(now)
			if len(l.buckets) >= l.maxKeys {
				return false, l.window
			}
		}
		bucket = tokenBucket{tokens: l.capacity, last: now}
	}

	bucket = l.refill(bucket, now)
	if bucket.tokens < 1 {
		l.buckets[key] = bucket
		missing := 1 - bucket.tokens
		return false, time.Duration(missing / l.capacity * float64(l.window))
	}

	bucket.tokens--
	l.buckets[key] = bucket
	return true, 0
}

func (l *TokenBucketLimiter) refill(bucket tokenBucket, now time.Time) tokenBucket {
	if bucket.last.IsZero() || now.Before(bucket.last) {
		bucket.last = now
		return bucket
	}
	elapsed := now.Sub(bucket.last)
	if elapsed > 0 {
		bucket.tokens += elapsed.Seconds() / l.window.Seconds() * l.capacity
		if bucket.tokens > l.capacity {
			bucket.tokens = l.capacity
		}
		bucket.last = now
	}
	return bucket
}

func (l *TokenBucketLimiter) pruneIdleLocked(now time.Time) {
	for key, bucket := range l.buckets {
		bucket = l.refill(bucket, now)
		if bucket.tokens >= l.capacity && now.Sub(bucket.last) >= 0 {
			delete(l.buckets, key)
		}
	}
}

// ConnectionLimiter bounds long-lived connections globally and per key. Its
// per-key map cannot grow beyond maxTotal active connections.
type ConnectionLimiter struct {
	mu        sync.Mutex
	maxTotal  int
	maxPerKey int
	total     int
	perKey    map[string]int
}

func NewConnectionLimiter(maxTotal, maxPerKey int) *ConnectionLimiter {
	if maxTotal <= 0 || maxPerKey <= 0 {
		panic("trafficx: connection limits must be positive")
	}
	if maxPerKey > maxTotal {
		panic("trafficx: per-key connection limit cannot exceed total limit")
	}
	return &ConnectionLimiter{
		maxTotal:  maxTotal,
		maxPerKey: maxPerKey,
		perKey:    make(map[string]int),
	}
}

func (l *ConnectionLimiter) TryAcquire(key string) bool {
	if l == nil {
		return true
	}
	if key == "" {
		key = "_"
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.total >= l.maxTotal || l.perKey[key] >= l.maxPerKey {
		return false
	}
	l.total++
	l.perKey[key]++
	return true
}

func (l *ConnectionLimiter) Release(key string) {
	if l == nil {
		return
	}
	if key == "" {
		key = "_"
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	count := l.perKey[key]
	if count <= 0 || l.total <= 0 {
		panic("trafficx: connection release without acquire")
	}
	if count == 1 {
		delete(l.perKey, key)
	} else {
		l.perKey[key] = count - 1
	}
	l.total--
}

func (l *ConnectionLimiter) Active() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.total
}
