package repo

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRateLimiterAcquireRelease(t *testing.T) {
	rl := NewRateLimiter(2, 50)

	release1 := rl.Acquire()
	release2 := rl.Acquire()

	// Verify we can't acquire a third slot without blocking
	acquired := make(chan struct{}, 1)
	go func() {
		rel := rl.Acquire()
		acquired <- struct{}{}
		rel()
	}()

	select {
	case <-acquired:
		t.Fatal("should not acquire third slot while two are held")
	case <-time.After(50 * time.Millisecond):
		// expected: blocked
	}

	release1()

	select {
	case <-acquired:
		// expected: now acquired
	case <-time.After(1 * time.Second):
		t.Fatal("should have acquired slot after release")
	}

	release2()
}

func TestRateLimiterConcurrentSlots(t *testing.T) {
	maxConcurrent := 3
	rl := NewRateLimiter(maxConcurrent, 50)

	var running atomic.Int32
	var maxSeen atomic.Int32
	var wg sync.WaitGroup

	taskCount := 10
	wg.Add(taskCount)

	for i := 0; i < taskCount; i++ {
		go func() {
			defer wg.Done()
			release := rl.Acquire()
			defer release()

			cur := running.Add(1)
			for {
				old := maxSeen.Load()
				if cur <= old {
					break
				}
				if maxSeen.CompareAndSwap(old, cur) {
					break
				}
			}

			time.Sleep(10 * time.Millisecond)
			running.Add(-1)
		}()
	}

	wg.Wait()

	observed := maxSeen.Load()
	if int32(maxConcurrent) < observed {
		t.Errorf("max concurrent exceeded: limit=%d, observed=%d", maxConcurrent, observed)
	}
}

func TestHandleRetryAfterSeconds(t *testing.T) {
	rl := NewRateLimiter(1, 50)

	dur, err := rl.HandleRetryAfter("30")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if 30*time.Second != dur {
		t.Errorf("expected 30s, got %s", dur)
	}
}

func TestHandleRetryAfterZero(t *testing.T) {
	rl := NewRateLimiter(1, 50)

	dur, err := rl.HandleRetryAfter("0")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if 0 != dur {
		t.Errorf("expected 0, got %s", dur)
	}
}

func TestHandleRetryAfterNegative(t *testing.T) {
	rl := NewRateLimiter(1, 50)

	dur, err := rl.HandleRetryAfter("-5")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if 0 != dur {
		t.Errorf("expected 0 for negative, got %s", dur)
	}
}

func TestHandleRetryAfterHTTPDate(t *testing.T) {
	rl := NewRateLimiter(1, 50)

	future := time.Now().Add(60 * time.Second).UTC().Format(time.RFC1123)
	dur, err := rl.HandleRetryAfter(future)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if dur < 55*time.Second || dur > 65*time.Second {
		t.Errorf("expected ~60s, got %s", dur)
	}
}

func TestHandleRetryAfterPastDate(t *testing.T) {
	rl := NewRateLimiter(1, 50)

	past := time.Now().Add(-60 * time.Second).UTC().Format(time.RFC1123)
	dur, err := rl.HandleRetryAfter(past)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if 0 != dur {
		t.Errorf("expected 0 for past date, got %s", dur)
	}
}

func TestHandleRetryAfterEmpty(t *testing.T) {
	rl := NewRateLimiter(1, 50)

	_, err := rl.HandleRetryAfter("")
	if nil == err {
		t.Fatal("expected error for empty header")
	}
}

func TestHandleRetryAfterInvalid(t *testing.T) {
	rl := NewRateLimiter(1, 50)

	_, err := rl.HandleRetryAfter("not-a-number-or-date")
	if nil == err {
		t.Fatal("expected error for invalid header")
	}
}

func TestWaitForHostWithRPM(t *testing.T) {
	rl := NewRateLimiter(5, 50)

	// Should not block for initial requests
	err := rl.WaitForHostWithRPM("example.com", 100)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	err = rl.WaitForHostWithRPM("example.com", 100)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForHostDifferentHosts(t *testing.T) {
	rl := NewRateLimiter(5, 50)

	err := rl.WaitForHostWithRPM("host-a.com", 100)
	if nil != err {
		t.Fatalf("unexpected error for host-a: %v", err)
	}

	err = rl.WaitForHostWithRPM("host-b.com", 100)
	if nil != err {
		t.Fatalf("unexpected error for host-b: %v", err)
	}

	// Verify they have separate limiters
	rl.mu.Lock()
	_, hasA := rl.limiters["host-a.com"]
	_, hasB := rl.limiters["host-b.com"]
	rl.mu.Unlock()

	if !hasA {
		t.Error("expected limiter for host-a.com")
	}
	if !hasB {
		t.Error("expected limiter for host-b.com")
	}
}
