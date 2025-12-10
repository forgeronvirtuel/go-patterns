package main

import (
	"fmt"
	"time"

	"forgeronvirtuel.com/gopatterns/internal/workerpool"
)

func main() {
	// Create a pool with 2 workers.
	pool := workerpool.NewWorkerPool(2)
	fmt.Println("Initial pool size:", pool.Size())

	// Define a simple task.
	task := func(id int) func() {
		return func() {
			fmt.Printf("Task %d started (pool size: %d)\n", id, pool.Size())
			time.Sleep(1 * time.Second)
			fmt.Printf("Task %d finished\n", id)
		}
	}

	// Run a few tasks sequentially (each Do waits for completion).
	for i := 1; i <= 3; i++ {
		if err := pool.Do(task(i)); err != nil {
			fmt.Println("Error:", err)
		}
	}

	// Grow the pool by 3 workers (total = 5).
	fmt.Println("Growing pool by 3...")
	pool.Grow(3)
	fmt.Println("New pool size:", pool.Size())

	// Launch several tasks sequentially (still via Do, blocking per task).
	for i := 4; i <= 6; i++ {
		if err := pool.Do(task(i)); err != nil {
			fmt.Println("Error:", err)
		}
	}

	// Shrink the pool by 4 workers.
	fmt.Println("Shrinking pool by 4...")
	pool.Shrink(4)
	fmt.Println("Pool size after shrink:", pool.Size())

	// Final task.
	if err := pool.Do(task(7)); err != nil {
		fmt.Println("Error:", err)
	}

	// Stop the pool.
	fmt.Println("Stopping pool...")
	pool.Stop()
	fmt.Println("Pool stopped. Exiting.")
}
