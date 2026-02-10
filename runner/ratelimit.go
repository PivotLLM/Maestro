/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package runner

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	maxRequests   int
	periodSeconds int
	requests      []time.Time
	mu            sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxRequests, periodSeconds int) *RateLimiter {
	return &RateLimiter{
		maxRequests:   maxRequests,
		periodSeconds: periodSeconds,
		requests:      make([]time.Time, 0, maxRequests),
	}
}

// Wait blocks until the rate limit allows a new request
// Returns the time waited
func (r *RateLimiter) Wait() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Duration(r.periodSeconds) * time.Second)

	// Remove expired requests
	validRequests := make([]time.Time, 0, len(r.requests))
	for _, t := range r.requests {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}
	r.requests = validRequests

	// If under limit, allow immediately
	if len(r.requests) < r.maxRequests {
		r.requests = append(r.requests, now)
		return 0
	}

	// Calculate wait time until oldest request expires
	oldestRequest := r.requests[0]
	waitDuration := oldestRequest.Add(time.Duration(r.periodSeconds) * time.Second).Sub(now)

	// Actually wait (release lock during wait)
	if waitDuration > 0 {
		r.mu.Unlock()
		time.Sleep(waitDuration)
		r.mu.Lock()

		// Re-record this request after waiting
		now = time.Now()
	}

	// Clean up again after wait
	cutoff = now.Add(-time.Duration(r.periodSeconds) * time.Second)
	validRequests = make([]time.Time, 0, len(r.requests))
	for _, t := range r.requests {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}
	r.requests = validRequests
	r.requests = append(r.requests, now)

	return waitDuration
}

// Available returns the number of requests available before hitting the limit
func (r *RateLimiter) Available() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Duration(r.periodSeconds) * time.Second)

	count := 0
	for _, t := range r.requests {
		if t.After(cutoff) {
			count++
		}
	}

	return r.maxRequests - count
}
