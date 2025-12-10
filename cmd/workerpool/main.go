package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"forgeronvirtuel.com/gopatterns/internal/workerpool"
)

func main() {
	// Create a pool with 3 workers.
	pool := workerpool.NewWorkerPool(3, 0)
	fmt.Println("Pool size:", pool.Size())

	// Example 1: synchronous Do (blocking)
	fmt.Println("=== Synchronous Do ===")
	err := pool.Do(func() error {
		fmt.Println("Sync task started")
		time.Sleep(1 * time.Second)
		fmt.Println("Sync task finished")
		return nil
	})
	if err != nil {
		fmt.Println("Error:", err)
	}

	// Example 2: asynchronous Submit
	fmt.Println("=== Asynchronous Submit ===")

	var jobs []*workerpool.Job

	for i := 1; i <= 5; i++ {
		i := i // capture
		job, err := pool.Submit(func() error {
			fmt.Printf("Async task %d started\n", i)
			time.Sleep(1 * time.Second)
			fmt.Printf("Async task %d finished\n", i)
			return nil
		})
		if err != nil {
			fmt.Println("Submit error:", err)
			continue
		}
		jobs = append(jobs, job)
	}

	// Do something else while tasks are running...
	fmt.Println("Main is free to do other work while tasks run...")

	// Wait for all async jobs to complete.
	for idx, job := range jobs {
		if err := job.Wait(); err != nil {
			fmt.Printf("Job %d finished with error: %v\n", idx+1, err)
		} else {
			fmt.Printf("Job %d finished successfully\n", idx+1)
		}
	}

	fmt.Println("=== WorkerPool experiment ===")

	// Slow task (simulates heavy processing)
	slowTask := func(id int) workerpool.TaskFunc {
		return func() error {
			fmt.Printf("[%s] Worker executing task %d...\n", ts(), id)
			time.Sleep(2 * time.Second)
			fmt.Printf("[%s] Worker finished task %d.\n", ts(), id)
			return nil
		}
	}

	fmt.Println("\n--- 1) Submit (blocking) demonstration ---")
	go func() {
		time.Sleep(500 * time.Millisecond)
		fmt.Printf("[%s] Unblocking Submit because a worker has started.\n", ts())
	}()

	fmt.Printf("[%s] Sending blocking task #1...\n", ts())
	pool.Submit(slowTask(1)) // should run immediately

	fmt.Printf("[%s] Sending blocking task #2...\n", ts())
	pool.Submit(slowTask(2)) // will wait until worker is free (if unbuffered)

	fmt.Println("\n--- 2) TrySubmit (non-blocking) demonstration ---")

	for i := 3; i <= 7; i++ {
		fmt.Printf("[%s] Sending TrySubmit task %d...\n", ts(), i)

		job, err := pool.TrySubmit(slowTask(i))
		if err != nil {
			fmt.Printf("[%s] TrySubmit(%d) FAILED: %v\n", ts(), i, err)
			continue
		}

		go func(jobID int, j *workerpool.Job) {
			_ = j.Wait()
			fmt.Printf("[%s] Task %d completed (via TrySubmit)\n", ts(), jobID)
		}(i, job)
	}

	fmt.Println("\n--- 3) Main is doing other work ---")
	for i := 0; i < 5; i++ {
		fmt.Printf("[%s] main() doing something...\n", ts())
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("=== WorkerPool demonstration: Context-aware tasks ===")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	job, err := pool.SubmitWithContext(ctx, func(ctx context.Context) error {
		for range 5 {
			select {
			case <-ctx.Done():
				fmt.Println("Task cancelled:", ctx.Err())
				return ctx.Err()
			default:
				fmt.Println("Working...")
				time.Sleep(500 * time.Millisecond)
			}
		}
		return nil
	})

	if err != nil {
		log.Fatal(err)
	}

	err = job.Wait()
	fmt.Println("Final error from job:", err)

	fmt.Println("\nStopping pool...")
	pool.Stop()
	fmt.Println("Pool stopped. Exiting.")
}

func ts() string {
	return time.Now().Format("15:04:05.000")
}
