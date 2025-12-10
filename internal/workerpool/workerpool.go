package workerpool

import (
	"sync"
)

// Worker represents a single goroutine that can execute submitted tasks.
type Worker struct {
	tasks chan func() // channel on which the main program sends tasks
	wg    sync.WaitGroup
}

// NewWorker creates a new Worker instance.
func NewWorker() *Worker {
	return &Worker{
		tasks: make(chan func()),
	}
}

// Start launches the worker goroutine.
// It listens on the tasks channel and executes incoming functions.
func (w *Worker) Start() {
	w.wg.Add(1)

	go func() {
		defer w.wg.Done()

		for task := range w.tasks {
			if task == nil {
				// Just in case, ignore nil tasks.
				continue
			}
			task()
		}
	}()
}

// Do sends a task to the worker and blocks until the task is finished.
// The caller provides the task as a function with no arguments and no return value.
func (w *Worker) Do(task func()) {
	// done channel is used to signal that the task has completed.
	done := make(chan struct{})

	// Wrap the user task to signal completion.
	wrappedTask := func() {
		defer close(done)
		task()
	}

	// Send the wrapped task to the worker.
	w.tasks <- wrappedTask

	// Block here until the wrapped task has signaled completion.
	<-done
}

// Stop closes the tasks channel and waits for the worker goroutine to exit.
func (w *Worker) Stop() {
	// Closing the channel will cause the worker's for-range loop to exit.
	close(w.tasks)

	// Wait for the worker goroutine to finish.
	w.wg.Wait()
}
