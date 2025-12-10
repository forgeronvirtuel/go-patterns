package workerpool

import (
	"errors"
	"sync"
)

// WorkerPool represents a dynamically resizable pool of worker goroutines.
type WorkerPool struct {
	jobs   chan func()
	wg     sync.WaitGroup
	mu     sync.Mutex
	size   int
	closed bool
}

// NewWorkerPool creates a new worker pool with the given initial size.
// Initial size can be 0 (lazy grow).
func NewWorkerPool(initialSize int) *WorkerPool {
	if initialSize < 0 {
		initialSize = 0
	}

	p := &WorkerPool{
		jobs: make(chan func()),
	}
	p.Grow(initialSize)
	return p
}

// worker is the function executed by each worker goroutine.
func (p *WorkerPool) worker() {
	defer p.wg.Done()

	for {
		task, ok := <-p.jobs
		if !ok {
			// Channel closed: pool is stopping
			return
		}
		if task == nil {
			// Nil task is a signal for this worker to exit.
			return
		}
		task()
	}
}

// Grow increases the number of workers by n.
func (p *WorkerPool) Grow(n int) {
	if n <= 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	for i := 0; i < n; i++ {
		p.wg.Add(1)
		go p.worker()
		p.size++
	}
}

// Shrink decreases the number of workers by n (at most current size).
// This is done by sending "nil" tasks that tell workers to exit.
func (p *WorkerPool) Shrink(n int) {
	if n <= 0 {
		return
	}

	p.mu.Lock()
	if n > p.size {
		n = p.size
	}
	if n <= 0 || p.closed {
		p.mu.Unlock()
		return
	}
	p.size -= n
	p.mu.Unlock()

	// Send "exit signals" to n workers.
	for i := 0; i < n; i++ {
		p.jobs <- nil
	}
}

// Do sends a task to the pool and waits until it is finished.
// Returns an error if the pool is already stopped.
func (p *WorkerPool) Do(task func()) error {
	if task == nil {
		return errors.New("task cannot be nil")
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return errors.New("worker pool is stopped")
	}
	p.mu.Unlock()

	done := make(chan struct{})

	wrapped := func() {
		defer close(done)
		task()
	}

	// Send the task to the workers.
	p.jobs <- wrapped

	// Block until the task is completed.
	<-done
	return nil
}

// Stop stops the pool completely: closes the jobs channel and waits
// for all workers to exit.
func (p *WorkerPool) Stop() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()

	// Closing the channel will make all workers exit their loop.
	close(p.jobs)

	// Wait until all workers are done.
	p.wg.Wait()
}

// Size returns the current number of workers (approximate, but protected by a mutex).
func (p *WorkerPool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.size
}
