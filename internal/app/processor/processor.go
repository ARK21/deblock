package processor

import (
	"context"
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/ARK21/deblock/internal/app/filter"
	"github.com/ARK21/deblock/internal/app/kafka"
	"github.com/ARK21/deblock/internal/app/rpc"
	"github.com/ThreeDotsLabs/watermill/components/cqrs"
	"github.com/ethereum/go-ethereum/common"
)

type Service struct {
	RPC      rpc.Client
	Matcher  *filter.Matcher
	EventBus *cqrs.EventBus
	ChainID  uint64
}

func NewService(rpcClient rpc.Client, matcher *filter.Matcher, eventBus *cqrs.EventBus, chainID uint64) *Service {
	if rpcClient == nil {
		panic("RPC client cannot be nil")
	}
	if matcher == nil {
		panic("Matcher cannot be nil")
	}
	if eventBus == nil {
		panic("EventBus cannot be nil")
	}

	return &Service{
		RPC:      rpcClient,
		Matcher:  matcher,
		EventBus: eventBus,
		ChainID:  chainID,
	}
}

func (s *Service) ProcessBlock(ctx context.Context, blk rpc.Block, reorged bool) (int, error) {
	type match struct {
		tx  rpc.Tx
		in  string
		out string
	}
	var ms []match
	for _, tx := range blk.Txs {
		fromUID, toUID, ok := s.Matcher.Match(tx.From, tx.To)
		if !ok {
			continue
		}
		ms = append(ms, match{tx: tx, in: toUID, out: fromUID})
	}
	if len(ms) == 0 {
		return 0, nil
	}

	//Batch receipts
	hashes := make([]string, len(ms))
	seen := make(map[string]struct{}, len(ms))
	for _, m := range ms {
		if _, ok := seen[m.tx.Hash]; !ok {
			seen[m.tx.Hash] = struct{}{}
			hashes = append(hashes, m.tx.Hash)
		}
	}

	ctxTO, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	receipts, err := s.RPC.BatchGetReceipts(ctxTO, hashes)
	if err != nil {
		// fallback (rare): fetch individually with small concurrency
		receipts = make(map[string]rpc.Receipt, len(hashes))
		sem := make(chan struct{}, 8) // limit concurrency to 10
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, hash := range hashes {
			wg.Add(1)
			sem <- struct{}{}
			go func(h string) {
				defer wg.Done()
				defer func() { <-sem }()
				rctx, cc := context.WithTimeout(ctxTO, 5*time.Second)
				defer cc()
				if r, e := s.RPC.GetTxReceipt(rctx, h); e == nil {
					mu.Lock()
					receipts[h] = r
					mu.Unlock()
				} else {
					log.Printf("failed to get receipt for %s: %v", h, e)
				}
			}(hash)
		}
		wg.Wait()
	}

	for _, m := range ms {
		rcpt, ok := receipts[m.tx.Hash]
		if !ok {
			log.Printf("no receipt for tx %s in block %d", m.tx.Hash, blk.Number)
			continue
		}

		from := common.HexToAddress(m.tx.From).Hex()
		to := ""
		if m.tx.To != nil {
			to = common.HexToAddress(*m.tx.To).Hex()
		}

		amountWei := strToBig(m.tx.Value)
		feeWei := big.NewInt(0)
		gasUsed := strToBig(rcpt.GasUsed)
		egp := strToBig(rcpt.EffectiveGasPrice)
		if gasUsed.Sign() > 0 && egp.Sign() > 0 {
			feeWei = new(big.Int).Mul(feeWei, egp)
		}

		status := "reverted"
		if rcpt.Status == 1 {
			status = "success"
		}

		event := kafka.MatchedTxEvent{
			TxHash:      m.tx.Hash,
			BlockNumber: blk.Number,
			BlockTime:   int64(blk.Timestamp),
			From:        from,
			To:          to,
			AmountWei:   amountWei.String(),
			AmountEth:   weiToEth(amountWei),
			FeeWei:      feeWei.String(),
			FeeEth:      weiToEth(feeWei),
			Status:      status,
			ChainID:     s.ChainID,
			Reorged:     reorged,
		}

		// Emit for incoming
		if m.in != "" {
			ie := event
			ie.Header = kafka.NewMessageHeader("MatchedTxEvent")
			ie.UserID = m.in
			ie.Address = to
			ie.Direction = "in"

			if err = s.EventBus.Publish(ctx, ie); err != nil {
				log.Printf("failed to publish event: %v", err)
			}
		}

		// Emit for outgoing
		if m.out != "" {
			oe := event
			oe.Header = kafka.NewMessageHeader("MatchedTxEvent")
			oe.UserID = m.out
			oe.Address = from
			oe.Direction = "out"

			if err = s.EventBus.Publish(ctx, oe); err != nil {
				log.Printf("failed to publish event: %v", err)
			}
		}

	}
	return len(ms), nil
}

func weiToEth(wei *big.Int) string {
	if wei == nil {
		return "0"
	}
	// return decimal string with 18 fractional digits
	// ETH = wei / 1e18
	denom := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	r := new(big.Rat).SetFrac(wei, denom.Num())
	return r.FloatString(18)
}

func getReceiptWithTRetry(ctx context.Context, c rpc.Client, hash string, n int, backoff time.Duration) (rpc.Receipt, error) {
	var rr rpc.Receipt
	var err error
	for i := 0; i < n; i++ {
		rr, err = c.GetTxReceipt(ctx, hash)
		if err == nil {
			return rr, nil
		}
		select {
		case <-ctx.Done():
			return rr, ctx.Err()
		case <-time.After(backoff):
			backoff = backoff * 2
		}
	}
	return rr, err
}

func strToBig(s string) *big.Int {
	if s == "" {
		return big.NewInt(0)
	}
	if len(s) > 2 && s[:2] == "0x" {
		n := new(big.Int)
		n.SetString(s[2:], 16)
		return n
	}
	n := new(big.Int)
	n.SetString(s, 10)
	return n
}
