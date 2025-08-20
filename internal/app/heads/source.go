package heads

import (
	"context"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/ARK21/deblock/internal/app/metrics"
	"github.com/ARK21/deblock/internal/app/rpc"
)

// Source emits heads from WS when available; on WS failure it polls HTTP.
// It deduplicates by block number and only moves forward.
type Source struct {
	Client         rpc.Client
	PollInterval   time.Duration
	ReconnectFloor time.Duration
	ReconnectCeil  time.Duration
}

func NewSource(
	client rpc.Client,
	pollInterval time.Duration,
	reconnectFloor time.Duration,
	reconnectCeil time.Duration,
) *Source {
	if pollInterval <= 0 {
		pollInterval = 3 * time.Second
	}
	if reconnectFloor <= 0 {
		reconnectFloor = time.Second
	}
	if reconnectCeil <= 0 {
		reconnectCeil = 30 * time.Second

	}

	return &Source{
		Client:         client,
		PollInterval:   pollInterval,
		ReconnectFloor: reconnectFloor,
		ReconnectCeil:  reconnectCeil,
	}
}

func (s *Source) Run(ctx context.Context) (<-chan rpc.Header, <-chan error) {
	out := make(chan rpc.Header, 128)
	errs := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errs)
		var last uint64
		backoff := s.ReconnectFloor
		ceil := s.ReconnectCeil
		poll := s.PollInterval

		for ctx.Err() == nil {
			heads, errors := s.Client.SubscribeNewHeads(ctx)
			if heads != nil {
				log.Printf("[heads] ws subscribed")
				metrics.SetWSUp(true)
				// While WS OK, forward and reset backoff
				for {
					select {
					case h, ok := <-heads:
						if !ok {
							heads = nil
							break
						}
						if h.Number > last {
							metrics.SetHead(h.Number)
							out <- h
							last = h.Number
						}
					case err := <-errors:
						errs <- err
						log.Printf("[heads] ws error: %s", err)
						heads = nil
					case <-ctx.Done():
						return
					}
					if heads == nil {
						break
					}
					backoff = s.ReconnectFloor
				}
			}
			log.Printf("[heads] fallback to HTTP polling every %s", poll)
			metrics.SetWSUp(false)
			t := time.NewTicker(poll)
			for {
				select {
				case <-t.C:
					num, err := s.Client.GetBlockNumber(ctx)
					if err != nil {
						errs <- err
						// sleep with backoff+jitter before retrying poll
						sleep := jitter(backoff)
						if backoff < ceil {
							backoff = minDur(ceil, time.Duration(float64(backoff)*1.6))
						}
						time.Sleep(sleep)
						continue
					}
					// emit any missed numbers
					for n := last + 1; n <= num; n++ {
						out <- rpc.Header{Number: n}
						last = n
					}
					// opportunistically try WS again sometimes
					// (cheap option: after each successful poll tick)
					goto TRY_WS
				case <-ctx.Done():
					t.Stop()
					return
				}
			}
		TRY_WS:
			t.Stop()
			// slight jitter before attempting WS resubscribe
			time.Sleep(jitter(backoff))
			if backoff < ceil {
				backoff = minDur(ceil, time.Duration(float64(backoff)*1.6))
			}
		}
	}()

	return out, errs
}

func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		d = time.Second
	}
	// 50% ± random(0..100%) jitter
	f := 0.5 + rand.Float64()
	return time.Duration(math.Round(float64(d) * f))
}

func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
