package workerpool

import (
	"errors"
	"sync"
)

// TaskFunc is the function type executed by workers.
// It can return an error that will be accessible from the Job handle.
type TaskFunc func() error

// WorkerPool represents a dynamically resizable pool of worker goroutines.
type WorkerPool struct {
	jobs   chan *Job
	wg     sync.WaitGroup
	mu     sync.Mutex
	size   int
	closed bool
}

// NewWorkerPool creates a new worker pool with the given initial size.
// Initial size can be 0 (lazy grow).
func NewWorkerPool(initialSize int, queueCapacity int) *WorkerPool {
	if initialSize < 0 {
		initialSize = 0
	}

	p := &WorkerPool{
		jobs: make(chan *Job, queueCapacity),
	}
	p.Grow(initialSize)
	return p
}

// worker is the function executed by each worker goroutine.
func (p *WorkerPool) worker() {
	defer p.wg.Done()

	for {
		job, ok := <-p.jobs
		if !ok {
			// Channel closed: pool is stopping
			return
		}
		if job == nil {
			// Nil task is a signal for this worker to exit.
			return
		}
		job.wrapper()
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

// Submit sends a task to the pool and returns a Job handle immediately
// without waiting for completion. The caller can later call job.Wait()
// or use job.Done() to be notified when the task is finished.
//
// Returns an error if the pool is already stopped.
func (p *WorkerPool) Submit(task TaskFunc) (*Job, error) {
	if task == nil {
		return nil, errors.New("task cannot be nil")
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, errors.New("worker pool is stopped")
	}
	p.mu.Unlock()

	job := &Job{
		done: make(chan struct{}),
		task: task,
	}

	// Enqueue the job. This may block if the jobs channel is unbuffered
	// and there are no workers currently ready to receive.
	p.jobs <- job

	return job, nil
}

// TrySubmit sends a task to the pool without blocking.
// If the queue is full or the pool is stopped, it returns an error.
func (p *WorkerPool) TrySubmit(task TaskFunc) (*Job, error) {
	if task == nil {
		return nil, errors.New("task cannot be nil")
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, errors.New("worker pool is stopped")
	}
	jobsChan := p.jobs // copy reference while protected
	p.mu.Unlock()

	job := &Job{
		done: make(chan struct{}),
		task: task,
	}

	select {
	case jobsChan <- job:
		// ok
		return job, nil

	default:
		// queue full (or unbuffered channel currently blocked)
		return nil, ErrQueueFull
	}
}

// Do sends a task to the pool and blocks until it is finished.
// It is implemented on top of Submit.
func (p *WorkerPool) Do(task TaskFunc) error {
	job, err := p.Submit(task)
	if err != nil {
		return err
	}
	return job.Wait()
}

// TryDo sends a task to the pool without blocking and waits until it is finished.
// It is implemented on top of TrySubmit.
// If the queue is full or the pool is stopped, it returns an error.
func (p *WorkerPool) TryDo(task TaskFunc) error {
	job, err := p.TrySubmit(task)
	if err != nil {
		return err
	}
	return job.Wait()
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
