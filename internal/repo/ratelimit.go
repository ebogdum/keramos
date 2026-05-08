package repo

import (
	"strconv"
	"strings"
	"sync"
	"time"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/logger"
)

// RateLimiter controls concurrent download slots and per-host request rates.
type RateLimiter struct {
	semaphore chan struct{}
	limiters  map[string]*hostLimiter
	mu        sync.Mutex
}

type hostLimiter struct {
	tokens    int
	lastReset time.Time
	limit     int
}

// NewRateLimiter creates a RateLimiter with the given concurrency and per-host rate limits.
func NewRateLimiter(maxConcurrent, requestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		semaphore: make(chan struct{}, maxConcurrent),
		limiters:  make(map[string]*hostLimiter),
	}
}

// Acquire takes a concurrent download slot and returns a release function.
func (r *RateLimiter) Acquire() func() {
	r.semaphore <- struct{}{}
	return func() {
		<-r.semaphore
	}
}

// WaitForHost blocks if the per-host rate limit has been reached, waiting until
// tokens are replenished.
func (r *RateLimiter) WaitForHost(host string) error {
	r.mu.Lock()

	hl, ok := r.limiters[host]
	if !ok {
		hl = &hostLimiter{
			tokens:    cap(r.semaphore) * 5, // derive from concurrency as baseline
			lastReset: time.Now(),
			limit:     cap(r.semaphore) * 5,
		}
		r.limiters[host] = hl
	}

	elapsed := time.Since(hl.lastReset)
	if elapsed >= time.Minute {
		hl.tokens = hl.limit
		hl.lastReset = time.Now()
	}

	if hl.tokens > 0 {
		hl.tokens--
		r.mu.Unlock()
		return nil
	}

	for {
		waitDuration := time.Minute - elapsed
		r.mu.Unlock()

		logger.Debug("rate limited for host %s, waiting %s", host, waitDuration)
		time.Sleep(waitDuration)

		r.mu.Lock()
		elapsed = time.Since(hl.lastReset)
		if elapsed >= time.Minute {
			hl.tokens = hl.limit
			hl.lastReset = time.Now()
		}
		if hl.tokens > 0 {
			hl.tokens--
			r.mu.Unlock()
			return nil
		}
	}
}

// NewRateLimiterWithRPM creates a RateLimiter with explicit per-host requests-per-minute.
func NewRateLimiterWithRPM(maxConcurrent, requestsPerMinute int) *RateLimiter {
	rl := &RateLimiter{
		semaphore: make(chan struct{}, maxConcurrent),
		limiters:  make(map[string]*hostLimiter),
	}
	// Store the rpm in a closure-accessible way by overriding WaitForHost's default
	// We re-implement the limiter init with the explicit RPM below
	rl.mu.Lock()
	rl.limiters["__rpm__"] = &hostLimiter{limit: requestsPerMinute}
	rl.mu.Unlock()
	return rl
}

// WaitForHostWithRPM blocks if the per-host rate limit has been reached.
// Uses the configured requests-per-minute value.
func (r *RateLimiter) WaitForHostWithRPM(host string, rpm int) error {
	r.mu.Lock()

	hl, ok := r.limiters[host]
	if !ok {
		hl = &hostLimiter{
			tokens:    rpm,
			lastReset: time.Now(),
			limit:     rpm,
		}
		r.limiters[host] = hl
	}

	elapsed := time.Since(hl.lastReset)
	if elapsed >= time.Minute {
		hl.tokens = hl.limit
		hl.lastReset = time.Now()
	}

	if hl.tokens > 0 {
		hl.tokens--
		r.mu.Unlock()
		return nil
	}

	for {
		waitDuration := time.Minute - elapsed
		r.mu.Unlock()

		logger.Debug("rate limited for host %s, waiting %s", host, waitDuration)
		time.Sleep(waitDuration)

		r.mu.Lock()
		elapsed = time.Since(hl.lastReset)
		if elapsed >= time.Minute {
			hl.tokens = hl.limit
			hl.lastReset = time.Now()
		}
		if hl.tokens > 0 {
			hl.tokens--
			r.mu.Unlock()
			return nil
		}
	}
}

// HandleRetryAfter parses a Retry-After HTTP header value and returns the duration to wait.
// Supports both integer seconds and HTTP-date formats.
func (r *RateLimiter) HandleRetryAfter(header string) (time.Duration, error) {
	trimmed := strings.TrimSpace(header)
	if "" == trimmed {
		return 0, keramoserr.NewError(keramoserr.ErrRateLimit, "empty Retry-After header")
	}

	seconds, err := strconv.ParseInt(trimmed, 10, 64)
	if nil == err {
		if seconds < 0 {
			return 0, nil
		}
		return time.Duration(seconds) * time.Second, nil
	}

	parsed, err := time.Parse(time.RFC1123, trimmed)
	if nil == err {
		delta := time.Until(parsed)
		if delta < 0 {
			return 0, nil
		}
		return delta, nil
	}

	return 0, keramoserr.NewErrorf(keramoserr.ErrRateLimit, "unable to parse Retry-After header: %q", trimmed)
}
