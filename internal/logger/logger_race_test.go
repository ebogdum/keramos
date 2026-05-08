package logger

import (
	"sync"
	"testing"
)

func TestConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	iterations := 100

	// Concurrent Init calls
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			Init(0 == idx%2, 0 == idx%3)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = IsVerbose()
			_ = IsDebug()
		}()
	}

	// Concurrent log calls
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			Log("test %d", idx)
			Debug("test %d", idx)
			Warn("test %d", idx)
			Error("test %d", idx)
		}(i)
	}

	wg.Wait()
}
