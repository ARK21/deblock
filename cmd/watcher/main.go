package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ARK21/deblock/internal/app/config"
	"github.com/ARK21/deblock/internal/app/filter"
	"github.com/ARK21/deblock/internal/app/users"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	log.Println("Starting watcher...")

	conf := config.Read()

	u := users.LoadUsers(conf.UsersFile)
	m := filter.NewMatcher(u)

	fromUID, toUID, ok := m.Match("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", nil)
	fmt.Println("Matched? ", fromUID, toUID, ok)

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
