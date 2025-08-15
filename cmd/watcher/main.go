package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ARK21/deblock/internal/app/config"
	"github.com/ARK21/deblock/internal/app/filter"
	"github.com/ARK21/deblock/internal/app/heads"
	"github.com/ARK21/deblock/internal/app/rpc"
	"github.com/ARK21/deblock/internal/app/users"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
		log.Panicln("starting graceful shutdown")
		cancel()
	}()

	cnf := config.Read()

	u := users.LoadUsers(cnf.UsersFile)

	matcher := filter.NewMatcher(u)

	client, err := rpc.NewGethClient(ctx, cnf.WsURL, cnf.HttpUrl)
	if err != nil {
		log.Fatalf("rpc: %v", err)
	}

	finalizer := heads.NewFinalizer(cnf.Confirmations)
	hch, ech := client.SubscribeNewHeads(ctx)
	log.Printf("subscribed to newHeads (confs=%d)", cnf.Confirmations)

	for {
		select {
		case h, ok := <-hch:
			if !ok {
				log.Printf("heads channel closed: %v", hch)
				return
			}
			header := heads.Header{
				Hash:       h.Hash,
				ParentHash: h.ParentHash,
				Number:     h.Number,
			}
			fmt.Println("header", header)
			for _, fh := range finalizer.Add(header) {
				blk, err := client.GetBlockByNumber(ctx, fh.Number, true)

				if err != nil {
					log.Printf("get block by number %v: %v", fh.Number, err)
					continue
				}
				matches := 0
				for _, tx := range blk.Txs {
					if _, _, ok := matcher.Match(tx.From, tx.To); ok {
						matches++
					}
				}
				log.Printf("finalized block=%d txs=%d matches=%d", blk.Number, len(blk.Txs), matches)
			}
		case err := <-ech:
			log.Printf("newHeads err: %v", err)
		case <-ctx.Done():
			log.Printf("shutdown")
			return
		}
	}
}
