package workerpool

import "errors"

// ErrQueueFull is returned when submitting a job to a full queue.
var ErrQueueFull = errors.New("job queue is full")
