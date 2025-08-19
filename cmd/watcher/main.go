package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ARK21/deblock/internal/app/config"
	"github.com/ARK21/deblock/internal/app/filter"
	"github.com/ARK21/deblock/internal/app/heads"
	"github.com/ARK21/deblock/internal/app/kafka"
	"github.com/ARK21/deblock/internal/app/processor"
	"github.com/ARK21/deblock/internal/app/reorg"
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

	reorgMgr := reorg.NewManager(conf.ReorgDepth)

	for {
		select {
		case h, ok := <-hch:
			if !ok {
				log.Printf("heads channel closed: %v", hch)
				return
			}
			finalized := finalizer.Add(heads.Header{
				Hash:       h.Hash,
				ParentHash: h.ParentHash,
				Number:     h.Number,
			})
			for _, fh := range finalized {
				// fetch the finalized block on the *current* canonical head
				blk, err := client.GetBlockByNumber(ctx, fh.Number, true)
				if err != nil {
					log.Printf("get block by number %v: %v", fh.Number, err)
					continue
				}
				if reorgMgr.ParentOK(blk) {
					// normal path
					matches, _ := srv.ProcessBlock(ctx, blk, false)
					reorgMgr.Record(blk)
					log.Printf("finalized block=%d txs=%d matches=%d", blk.Number, len(blk.Txs), matches)
					continue
				}

				// reorg path
				log.Printf("[REORG] parent mismatch at block %d (parent=%s)", blk.Number, blk.ParentHash)
				ancNum, _, found := reorgMgr.CommonAncestor(ctx, client, blk.Hash, blk.Number)
				if !found {
					log.Printf("[REORG] ancestor not found within depth, skipping block %d for now", blk.Number)
					// safe choice: skip until deeper heads arrive; or fall back to processing anyway
					continue
				}

				from, to, _ := reorgMgr.RewindRange(ancNum)
				log.Printf("[REORG] ancestor=%d; rewinding (%d..%d] and reprocessing to current %d",
					ancNum, from, to, blk.Number)

				// 1) Clear our local view for numbers > ancestor (drop stale mapping)
				reorgMgr.ResetAbove(ancNum)
				// 2) Re-process new canonical blocks from ancestor+1 up to current finalized number
				for n := ancNum + 1; n <= fh.Number; n++ {
					nb, err := client.GetBlockByNumber(ctx, n, true)
					if err != nil {
						log.Printf("[REORG] fetch %d: %v", n, err)
						break
					}
					matches, _ := srv.ProcessBlock(ctx, nb, true)
					reorgMgr.Record(nb)
					log.Printf("[REORG] reprocessed block=%d matches=%d", nb.Number, matches)
				}
			}
		case err := <-ech:
			log.Printf("newHeads err: %v", err)
		case <-ctx.Done():
			log.Printf("shutdown")
			return
		}
	}
}
