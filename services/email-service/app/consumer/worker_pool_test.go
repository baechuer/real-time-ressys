package consumer

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWorkerPool(t *testing.T) {
	pool := NewWorkerPool(5)
	require.NotNil(t, pool)
	assert.Equal(t, 5, pool.workers)
	defer pool.Stop() // Clean up
}

func TestWorkerPool_Submit(t *testing.T) {
	pool := NewWorkerPool(2)
	defer pool.Stop()

	var wg sync.WaitGroup
	var mu sync.Mutex
	executed := make([]int, 0)

	// Submit 5 jobs
	for i := 0; i < 5; i++ {
		wg.Add(1)
		jobID := i
		pool.Submit(func() {
			defer wg.Done()
			mu.Lock()
			executed = append(executed, jobID)
			mu.Unlock()
		})
	}

	// Wait for all jobs to complete
	wg.Wait()

	// Verify all jobs executed
	assert.Len(t, executed, 5)
}

func TestWorkerPool_ConcurrentExecution(t *testing.T) {
	pool := NewWorkerPool(3)
	defer pool.Stop()

	var wg sync.WaitGroup
	var mu sync.Mutex
	activeJobs := 0
	maxActive := 0

	// Submit 10 jobs that take some time
	for i := 0; i < 10; i++ {
		wg.Add(1)
		pool.Submit(func() {
			defer wg.Done()
			mu.Lock()
			activeJobs++
			if activeJobs > maxActive {
				maxActive = activeJobs
			}
			mu.Unlock()

			// Simulate work
			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			activeJobs--
			mu.Unlock()
		})
	}

	wg.Wait()

	// With 3 workers, max active should be at most 3
	assert.LessOrEqual(t, maxActive, 3)
	assert.Equal(t, 0, activeJobs)
}

func TestWorkerPool_Stop(t *testing.T) {
	pool := NewWorkerPool(2)

	var wg sync.WaitGroup
	var mu sync.Mutex
	executed := 0

	// Submit some jobs
	for i := 0; i < 5; i++ {
		wg.Add(1)
		pool.Submit(func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			executed++
			mu.Unlock()
		})
	}

	// Wait for all jobs to complete first
	wg.Wait()

	// Then stop the pool
	pool.Stop()

	// Verify all jobs executed
	mu.Lock()
	count := executed
	mu.Unlock()
	assert.Equal(t, 5, count)
}

func TestWorkerPool_Wait(t *testing.T) {
	pool := NewWorkerPool(2)

	var wg sync.WaitGroup
	var mu sync.Mutex
	executed := 0

	// Submit jobs
	for i := 0; i < 3; i++ {
		wg.Add(1)
		pool.Submit(func() {
			defer wg.Done()
			time.Sleep(20 * time.Millisecond)
			mu.Lock()
			executed++
			mu.Unlock()
		})
	}

	// Wait for all jobs to complete first
	wg.Wait()

	// Then wait for pool to finish
	pool.Wait()

	// Verify all jobs executed
	mu.Lock()
	count := executed
	mu.Unlock()
	assert.Equal(t, 3, count)
}

func TestWorkerPool_StopPreventsNewJobs(t *testing.T) {
	pool := NewWorkerPool(1)

	// Submit a job first and wait for it
	var wg sync.WaitGroup
	wg.Add(1)
	pool.Submit(func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
	})
	wg.Wait()

	// Now stop the pool
	pool.Stop()

	// Try to submit a new job after stop - should be safe (no-op)
	executed := false
	pool.Submit(func() {
		executed = true
	})

	// Give it a moment
	time.Sleep(50 * time.Millisecond)

	// Job should not execute after stop
	assert.False(t, executed)
}

func TestWorkerPool_MultipleStops(t *testing.T) {
	pool := NewWorkerPool(2)

	// Multiple stops should be safe
	pool.Stop()
	pool.Stop()
	pool.Stop()

	// Should not panic
}

func TestWorkerPool_Ordering(t *testing.T) {
	// Note: Worker pools don't guarantee order
	// This test just verifies jobs execute
	pool := NewWorkerPool(2)
	defer pool.Stop()

	var wg sync.WaitGroup
	results := make([]int, 0)
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		val := i
		pool.Submit(func() {
			defer wg.Done()
			mu.Lock()
			results = append(results, val)
			mu.Unlock()
		})
	}

	wg.Wait()

	// All jobs should execute
	assert.Len(t, results, 10)
}

func TestWorkerPool_ZeroWorkers(t *testing.T) {
	pool := NewWorkerPool(0)
	require.NotNil(t, pool)

	// With 0 workers, no goroutines are started
	// Channel buffer is 0*2 = 0, so Submit will block
	// We should stop immediately without submitting jobs
	pool.Stop()

	// Pool should stop cleanly with 0 workers
	// (no workers to wait for)
}
