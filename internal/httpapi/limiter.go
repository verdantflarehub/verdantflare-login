package httpapi

import (
	"sync"
	"time"
)

type rateEntry struct {
	count   int
	resetAt time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]rateEntry
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{entries: make(map[string]rateEntry)}
}

func (l *rateLimiter) allow(key string, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	entry, exists := l.entries[key]
	if !exists || !now.Before(entry.resetAt) {
		l.entries[key] = rateEntry{count: 1, resetAt: now.Add(window)}
		return true
	}
	if entry.count >= limit {
		return false
	}
	entry.count++
	l.entries[key] = entry
	return true
}
