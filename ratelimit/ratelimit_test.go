package ratelimit

import (
	"testing"
	"time"
)

func TestTokenBucket_Allow(t *testing.T) {
	bucket := NewTokenBucket(10, 5) // 10 req/s, burst 5

	// Should allow burst up to capacity
	for i := 0; i < 5; i++ {
		if !bucket.Allow() {
			t.Errorf("request %d should be allowed (burst)", i+1)
		}
	}

	// Next request should be denied (exhausted burst)
	if bucket.Allow() {
		t.Error("request should be denied after burst exhausted")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	bucket := NewTokenBucket(100, 10) // 100 req/s, burst 10

	// Exhaust all tokens
	for i := 0; i < 10; i++ {
		bucket.Allow()
	}

	// Should be denied immediately
	if bucket.Allow() {
		t.Error("should be denied after exhausting tokens")
	}

	// Wait for refill (100 tokens/sec = 1 token per 10ms)
	time.Sleep(20 * time.Millisecond)

	// Should have refilled at least 1 token
	if !bucket.Allow() {
		t.Error("should be allowed after refill")
	}
}

func TestRateLimiter_MultipleClients(t *testing.T) {
	rl := NewRateLimiter(10, 2)

	// Client 1 exhausts burst
	rl.Allow("client1")
	rl.Allow("client1")
	if rl.Allow("client1") {
		t.Error("client1 should be rate limited")
	}

	// Client 2 should still have tokens
	if !rl.Allow("client2") {
		t.Error("client2 should not be rate limited")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(10, 5)
	
	// Add some clients
	rl.Allow("client1")
	rl.Allow("client2")
	
	// Cleanup shouldn't panic
	rl.Cleanup()
}
