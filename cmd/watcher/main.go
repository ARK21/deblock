package main

import (
	"context"
	"log"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	log.Println("Starting watcher...")

	// TODO: wire config, metrics and start pipelines next steps

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Println("alive")
		}
	}
}
