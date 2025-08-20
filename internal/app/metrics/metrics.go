package metrics

import (
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	//Gauges
	headBlock      = prometheus.NewGauge(prometheus.GaugeOpts{Name: "eth_head_block"})
	finalizedBlock = prometheus.NewGauge(prometheus.GaugeOpts{Name: "eth_finalized_block"})
	lagBlocks      = prometheus.NewGauge(prometheus.GaugeOpts{Name: "eth_finalized_lag_blocks"})
	wsConnected    = prometheus.NewGauge(prometheus.GaugeOpts{Name: "ws_connected"})

	inflightReceipts = prometheus.NewGauge(prometheus.GaugeOpts{Name: "rpc_receipts_inflight"})

	// Counters
	blockProcessed  = prometheus.NewCounter(prometheus.CounterOpts{Name: "block_processed_total"})
	reprocessed     = prometheus.NewCounter(prometheus.CounterOpts{Name: "block_reprocessed_total"})
	txsMatched      = prometheus.NewCounter(prometheus.CounterOpts{Name: "txs_matched_total"})
	eventsPublished = prometheus.NewCounter(prometheus.CounterOpts{Name: "events_published_total"})
	reorgsTotal     = prometheus.NewCounter(prometheus.CounterOpts{Name: "reorgs_total"})

	rpcCalls         = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "rpc_calls_total"}, []string{"method", "result"})
	receiptBatchSize = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "rpc_receipts_batch_size",
		Buckets: []float64{1, 5, 10, 20, 50, 100, 200},
	})
)

func init() {
	prometheus.MustRegister(
		headBlock,
		finalizedBlock,
		lagBlocks,
		wsConnected,
		inflightReceipts,

		blockProcessed,
		reprocessed,
		txsMatched,
		eventsPublished,
		reorgsTotal,

		rpcCalls,
		receiptBatchSize,
	)
}

var (
	lastHeadUnix      int64 // unix seconds
	lastFinalizedUnix int64
	lastRPCErrUnix    int64
	wsUp              uint32
)

func SetHead(n uint64) {
	headBlock.Set(float64(n))
	atomic.StoreInt64(&lastHeadUnix, time.Now().Unix())
	updateLag()
}

func SetFinalized(n uint64) {
	finalizedBlock.Set(float64(n))
	atomic.StoreInt64(&lastFinalizedUnix, time.Now().Unix())
	updateLag()
}

func SetWSUp(up bool) {
	if up {
		atomic.StoreUint32(&wsUp, 1)
		wsConnected.Set(1)
	} else {
		atomic.StoreUint32(&wsUp, 0)
		wsConnected.Set(0)
	}
}

func IncBlocksProcessed() {
	blockProcessed.Inc()
}

func IncReprocessed() {
	reprocessed.Inc()
}

func AddTxsMatched(n int) {
	if n > 0 {
		txsMatched.Add(float64(n))
	}
}

func AddEventsPublished(n int) {
	if n > 0 {
		eventsPublished.Add(float64(n))
	}
}

func IncReorg() {
	reorgsTotal.Inc()
}

func RPCCall(method string, ok bool) {
	rpcCalls.WithLabelValues(method, map[bool]string{true: "ok", false: "err"}[ok]).Inc()
	if !ok {
		atomic.StoreInt64(&lastRPCErrUnix, time.Now().Unix())
	}
}

func ReceiptsInFlight(n int) {
	inflightReceipts.Set(float64(n))
}

func ObserveReceiptBatch(size int) {
	if size > 0 {
		receiptBatchSize.Observe(float64(size))
	}
}

func updateLag() {
	h := headBlock.Desc() // sentinel to avoid unused; actual lag computed by reading gauge values below (kept simple here).
	_ = h
	// We can’t read gauge values directly; recompute from last seen head/finalized by storing numbers here if needed.
}

func IsHealthy(now time.Time) (ok bool, reason string) {
	headAge := now.Sub(time.Unix(atomic.LoadInt64(&lastHeadUnix), 0))
	finalAge := now.Sub(time.Unix(atomic.LoadInt64(&lastFinalizedUnix), 0))
	rpcErrAge := now.Sub(time.Unix(atomic.LoadInt64(&lastRPCErrUnix), 0))
	ws := atomic.LoadUint32(&wsUp) == 1

	// simple SLOs (tweak as needed)
	if headAge > 2*time.Minute && !ws {
		return false, "no heads for >2m and WS down"
	}
	if finalAge > 5*time.Minute {
		return false, "no finalized progress for >5m"
	}
	if rpcErrAge < 30*time.Second {
		return false, "recent rpc errors"
	}

	return true, "ok"
}
