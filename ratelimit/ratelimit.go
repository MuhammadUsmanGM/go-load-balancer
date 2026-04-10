package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket implements the token bucket rate limiting algorithm.
type TokenBucket struct {
	rate       float64     // tokens per second
	burst      int64       // max bucket capacity
	tokens     float64     // current tokens
	lastRefill time.Time   // last token refill time
	mu         sync.Mutex
}

// NewTokenBucket creates a new token bucket.
func NewTokenBucket(rate float64, burst int64) *TokenBucket {
	return &TokenBucket{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst), // Start full
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed.
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.lastRefill = now

	// Refill tokens
	tb.tokens += elapsed * tb.rate
	if tb.tokens > float64(tb.burst) {
		tb.tokens = float64(tb.burst)
	}

	// Consume token
	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}

	return false
}

// RateLimiter manages rate limiting for multiple clients.
type RateLimiter struct {
	clients map[string]*TokenBucket
	rate    float64
	burst   int64
	mu      sync.RWMutex
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(rate float64, burst int64) *RateLimiter {
	return &RateLimiter{
		clients: make(map[string]*TokenBucket),
		rate:    rate,
		burst:   burst,
	}
}

// Allow checks if a request from a client IP is allowed.
func (rl *RateLimiter) Allow(clientIP string) bool {
	rl.mu.RLock()
	bucket, exists := rl.clients[clientIP]
	rl.mu.RUnlock()

	if exists {
		return bucket.Allow()
	}

	// Create new bucket for this client
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if bucket, exists = rl.clients[clientIP]; exists {
		return bucket.Allow()
	}

	bucket = NewTokenBucket(rl.rate, rl.burst)
	rl.clients[clientIP] = bucket
	return bucket.Allow()
}

// Cleanup removes stale client entries.
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Simple cleanup - in production, you'd track last access time
	// For now, we keep all entries to avoid complexity
}
