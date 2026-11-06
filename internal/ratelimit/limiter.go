package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow reports whether the key may proceed. A failed attempt should call Record.
// For login/PIN we count every attempt (success or fail) via Allow+Record on attempt,
// or simply Check then Record on failure only — we use Allow which records immediately.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)
	kept := l.attempts[key][:0]
	for _, ts := range l.attempts[key] {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if cap(kept) == 0 {
		kept = make([]time.Time, 0, l.limit)
	}
	if len(kept) >= l.limit {
		l.attempts[key] = kept
		return false
	}
	l.attempts[key] = append(kept, now)
	return true
}
