package consumer

import (
	"sync"
)

// WorkerPool manages a pool of workers for concurrent message processing
type WorkerPool struct {
	workers    int
	jobs       chan func()
	wg         sync.WaitGroup
	stopOnce   sync.Once
	stopSignal chan struct{}
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers int) *WorkerPool {
	wp := &WorkerPool{
		workers:    workers,
		jobs:       make(chan func(), workers*2), // Buffer size
		stopSignal: make(chan struct{}),
	}

	// Start workers
	for i := 0; i < workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}

	return wp
}

// worker runs a single worker goroutine
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()

	for {
		select {
		case <-wp.stopSignal:
			return
		case job, ok := <-wp.jobs:
			if !ok {
				return
			}
			job()
		}
	}
}

// Submit submits a job to the worker pool
func (wp *WorkerPool) Submit(job func()) {
	select {
	case <-wp.stopSignal:
		// Pool is stopping, don't accept new jobs
		return
	default:
		// Try to submit job
		select {
		case <-wp.stopSignal:
			// Pool stopped while trying to submit
			return
		case wp.jobs <- job:
			// Job submitted successfully
		}
	}
}

// Wait waits for all workers to finish
func (wp *WorkerPool) Wait() {
	wp.stopOnce.Do(func() {
		close(wp.stopSignal)
		close(wp.jobs)
	})
	wp.wg.Wait()
}

// Stop stops the worker pool
func (wp *WorkerPool) Stop() {
	wp.Wait()
}
