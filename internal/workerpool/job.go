package workerpool

import "context"

type TaskFuncWithContext func(ctx context.Context) error

type Job struct {
	done   chan struct{}
	err    error
	task   TaskFunc
	ctask  TaskFuncWithContext
	ctx    context.Context
	cancel context.CancelFunc
}

// Wait blocks until the job is finished and returns the task error (if any).
func (j *Job) Wait() error {
	<-j.done
	return j.err
}

// Done returns a read-only channel that is closed when the job is finished.
// This is useful if you want to use select{} to wait on multiple jobs or timeouts.
func (j *Job) Done() <-chan struct{} {
	return j.done
}

// Wrap the task to set job.err and close the done channel when finished.
func (j *Job) wrapper() {
	// Execute the user task.
	if j.task != nil {
		j.err = j.task()
	}
	// Signal completion.
	close(j.done)
}

// Context-aware wrapper
func (j *Job) wrapperWithContext() {
	defer close(j.done)

	// Check if already cancelled before starting
	select {
	case <-j.ctx.Done():
		j.err = j.ctx.Err()
		return
	default:
	}

	// Execute the task
	if j.ctask != nil {
		j.err = j.ctask(j.ctx)
	}
}
