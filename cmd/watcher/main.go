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
	"github.com/ARK21/deblock/internal/app/kafka"
	"github.com/ARK21/deblock/internal/app/processor"
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

	conf := config.Read()

	u := users.LoadUsers(conf.UsersFile)

	matcher := filter.NewMatcher(u)

	client, err := rpc.NewGethClient(ctx, conf.WsURL, conf.HttpUrl)
	if err != nil {
		log.Fatalf("rpc: %v", err)
	}

	publisher, err := kafka.NewKafkaPublisher(conf.KafkaBrokers)
	if err != nil {
		log.Fatal("error creating kafka publisher:", err)
	}

	bus, err := kafka.NewEventBus(publisher, conf.KafkaTopic)
	if err != nil {
		log.Fatal("error creating event bus:", err)
	}

	chainID, err := client.GetChainID(ctx)
	if err != nil {
		log.Fatalf("get chain ID error: %v", err)
	}

	srv := processor.NewService(client, matcher, bus, chainID)

	finalizer := heads.NewFinalizer(conf.Confirmations)

	src := heads.NewSource(client, conf.HeadPollInterval, conf.WSReconnectFloor, conf.WSReconnectCeil)

	hch, ech := src.Run(ctx)
	log.Printf("subscribed to newHeads (confs=%d)", conf.Confirmations)

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
				matches, _ := srv.ProcessBlock(ctx, blk, false)
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
