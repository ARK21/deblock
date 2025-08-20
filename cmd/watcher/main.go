package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ARK21/deblock/internal/app/backfill"
	"github.com/ARK21/deblock/internal/app/checkpoint"
	"github.com/ARK21/deblock/internal/app/config"
	"github.com/ARK21/deblock/internal/app/filter"
	"github.com/ARK21/deblock/internal/app/heads"
	"github.com/ARK21/deblock/internal/app/kafka"
	"github.com/ARK21/deblock/internal/app/metrics"
	"github.com/ARK21/deblock/internal/app/processor"
	"github.com/ARK21/deblock/internal/app/reorg"
	"github.com/ARK21/deblock/internal/app/rpc"
	"github.com/ARK21/deblock/internal/app/users"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conf := config.Read()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ok, reason := metrics.IsHealthy(time.Now())
		if ok {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}
		http.Error(w, reason, http.StatusServiceUnavailable)
	})
	httpSrv := &http.Server{Addr: conf.HttpAddr, Handler: mux}

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("http server: %v", err)
		}
	}()

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
		log.Println("starting graceful shutdown")
		cancel()

		if err := httpSrv.Shutdown(ctx); err != nil {
			log.Fatalf("failed to shutdown server: %b", err)
		}
	}()

	fs := checkpoint.NewFileStore(conf.CheckpointFile)
	st, err := fs.Load(ctx)
	if err != nil {
		log.Fatalf("checkpoint load: %v", err)
	}
	log.Printf("checkpoint: last_finalized=%d", st.LastFinalized)

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
	reorgMgr := reorg.NewManager(conf.ReorgDepth)

	head, err := client.GetBlockNumber(ctx)
	if err != nil {
		log.Fatal("error getting block number:", err)
	}
	var target uint64
	if head >= uint64(conf.Confirmations) {
		target = head - uint64(conf.Confirmations)
	} else {
		target = 0
	}

	// If first run with no checkpoint, optionally limit bootstrap depth
	bootstrap := uint64(conf.BootstrapBlocks) // e.g., 0 (all) or 5000
	start := st.LastFinalized + 1
	if st.LastFinalized == 0 && bootstrap > 0 && target > bootstrap {
		start = target - bootstrap + 1
	}
	if start <= target {
		log.Printf("backfill: %d -> %d (target finalized)", start, target)
		var lastSaved time.Time
		save := func(n uint64) {
			// throttle saves to disk (e.g., every 250ms)
			if time.Since(lastSaved) < 250*time.Millisecond {
				return
			}
		_:
			fs.Save(ctx, checkpoint.State{LastFinalized: n, UpdatedAt: time.Now()})
			lastSaved = time.Now()
		}
		if err := backfill.Run(ctx, client, srv, reorgMgr, start, target, save); err != nil {
			log.Fatalf("backfill: %v", err)
		}
		// ensure final save
		_ = fs.Save(ctx, checkpoint.State{LastFinalized: target, UpdatedAt: time.Now()})
		log.Printf("backfill done up to %d", target)
	} else {
		log.Printf("no backfill needed (checkpoint at %d, target %d)", st.LastFinalized, target)
	}

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
					metrics.IncBlocksProcessed()
					metrics.AddTxsMatched(matches)
					metrics.SetFinalized(blk.Number)
					reorgMgr.Record(blk)
					_ = fs.Save(ctx, checkpoint.State{LastFinalized: blk.Number, UpdatedAt: time.Now()})
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

				// 1) Clear our local view for numbers > ancestor (drop stale mapping)
				reorgMgr.ResetAbove(ancNum)
				// 2) Re-process new canonical blocks from ancestor+1 up to current finalized number
				metrics.IncReorg()
				for n := ancNum + 1; n <= fh.Number; n++ {
					nb, err := client.GetBlockByNumber(ctx, n, true)
					if err != nil {
						log.Printf("[REORG] fetch %d: %v", n, err)
						break
					}
					matches, _ := srv.ProcessBlock(ctx, nb, true)
					reorgMgr.Record(nb)
					_ = fs.Save(ctx, checkpoint.State{LastFinalized: n, UpdatedAt: time.Now()})
					metrics.IncReprocessed()
					metrics.AddTxsMatched(matches)
					metrics.SetFinalized(nb.Number)
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
