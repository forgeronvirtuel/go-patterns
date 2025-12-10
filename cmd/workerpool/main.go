package main

import (
	"fmt"
	"time"

	"forgeronvirtuel.com/gopatterns/internal/workerpool"
)

func main() {
	// 1. Create the worker.
	worker := workerpool.NewWorker()

	// 2. Start the worker goroutine.
	worker.Start()

	// 3. Define a programmable task.
	myTask := func() {
		fmt.Println("Task started...")
		time.Sleep(2 * time.Second)
		fmt.Println("Task finished.")
	}

	// 4. Send the task to the goroutine and wait until it is done.
	fmt.Println("Sending task to worker...")
	worker.Do(myTask)
	fmt.Println("Task is confirmed finished in main.")

	// 5. Stop the worker (close the goroutine) and wait for it to end.
	fmt.Println("Stopping worker...")
	worker.Stop()
	fmt.Println("Worker stopped. Main exits.")
}
