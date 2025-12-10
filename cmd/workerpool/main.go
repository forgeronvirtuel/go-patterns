package main

import (
	"fmt"
	"time"

	"forgeronvirtuel.com/gopatterns/internal/workerpool"
)

func main() {
	// Create a pool with 3 workers.
	pool := workerpool.NewWorkerPool(3)
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

	// Stop the pool.
	fmt.Println("Stopping pool...")
	pool.Stop()
	fmt.Println("Pool stopped. Exiting.")
}
