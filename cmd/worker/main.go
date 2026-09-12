package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"filamenttracker/internal/worker"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler := worker.NewScheduler()
	if err := scheduler.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "worker failed: %v\n", err)
		log.Printf("worker exited with error: %v", err)
		os.Exit(1)
	}

	<-time.After(2 * time.Second)
}
