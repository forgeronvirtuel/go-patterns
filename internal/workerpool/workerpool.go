package workerpool

import (
	"context"
	"errors"
	"sync"
)

// TaskFunc is the function type executed by workers.
// It can return an error that will be accessible from the Job handle.
type TaskFunc func() error

// WorkerPool represents a dynamically resizable pool of worker goroutines.
type WorkerPool struct {
	jobsTransfer chan *Job
	jobs         chan *Job
	wg           sync.WaitGroup
	mu           sync.Mutex
	size         int
	closed       bool
}

// NewWorkerPool creates a new worker pool with the given initial size.
// Initial size can be 0 (lazy grow).
func NewWorkerPool(initialSize int, queueCapacity int) *WorkerPool {
	if initialSize < 0 {
		initialSize = 0
	}

	p := &WorkerPool{
		jobs:         make(chan *Job, queueCapacity),
		jobsTransfer: make(chan *Job),
	}
	p.Grow(initialSize)
	p.wg.Add(1)
	go transferJobs(p.jobs, p.jobsTransfer, &p.wg)
	return p
}

func transferJobs(src, dst chan *Job, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range src {
		dst <- job
	}
	close(dst)
}

// worker is the function executed by each worker goroutine.
func (p *WorkerPool) worker() {
	defer p.wg.Done()

	for {
		job, ok := <-p.jobsTransfer
		if !ok {
			// Channel closed: pool is stopping
			return
		}
		if job == nil {
			// Nil task is a signal for this worker to exit.
			return
		}
		// Choose wrapper type
		if job.ctask != nil {
			job.wrapperWithContext()
		} else {
			job.wrapper()
		}
	}
}

// ResizeTo changes the number of workers to the specified new size.
func (p *WorkerPool) ResizeTo(newSize int) {
	if newSize < 0 {
		newSize = 0
	}

	p.mu.Lock()
	currentSize := p.size
	p.mu.Unlock()
	if newSize > currentSize {
		p.Grow(newSize - currentSize)
	} else if newSize < currentSize {
		p.Shrink(currentSize - newSize)
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

// SubmitWithContext sends a context-aware task to the pool.
// Non-blocking cancellation is supported. The task must respect ctx.Done() inside its logic.
//
// If ctx is already cancelled, the job is not sent to the pool.
func (p *WorkerPool) SubmitWithContext(
	ctx context.Context,
	task TaskFuncWithContext,
) (*Job, error) {

	if task == nil {
		return nil, errors.New("task cannot be nil")
	}

	// Immediate cancellation?
	select {
	case <-ctx.Done():
		job := &Job{
			done: make(chan struct{}),
			err:  ctx.Err(),
		}
		close(job.done)
		return job, nil
	default:
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, errors.New("worker pool is stopped")
	}
	jobsChan := p.jobs
	p.mu.Unlock()

	// Create job
	cctx, cancel := context.WithCancel(ctx)
	job := &Job{
		done:   make(chan struct{}),
		ctask:  task,
		ctx:    cctx,
		cancel: cancel,
	}

	// Enqueue job
	jobsChan <- job

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
