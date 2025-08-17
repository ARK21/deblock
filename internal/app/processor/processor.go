package processor

import (
	"context"
	"log"
	"math/big"
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
	matches := 0
	for _, tx := range blk.Txs {
		fromUID, toUID, ok := s.Matcher.Match(tx.From, tx.To)
		if !ok {
			continue
		}
		matches++

		rcpt, err := getReceiptWithTRetry(ctx, s.RPC, tx.Hash, 3, 200*time.Millisecond)
		if err != nil {
			continue
		}

		from := common.HexToAddress(tx.From).Hex()
		to := ""
		if tx.To != nil {
			to = common.HexToAddress(*tx.To).Hex()
		}

		amountWei := strToBig(tx.Value)
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
			TxHash:      tx.Hash,
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
		if toUID != "" {
			ie := event
			ie.Header = kafka.NewMessageHeader("MatchedTxEvent")
			ie.UserID = toUID
			ie.Address = to
			ie.Direction = "in"

			if err := s.EventBus.Publish(ctx, ie); err != nil {
				log.Printf("failed to publish event: %v", err)
			}
		}

		// Emit for outgoing
		if fromUID != "" {
			oe := event
			oe.Header = kafka.NewMessageHeader("MatchedTxEvent")
			oe.UserID = fromUID
			oe.Address = from
			oe.Direction = "out"

			if err := s.EventBus.Publish(ctx, oe); err != nil {
				log.Printf("failed to publish event: %v", err)
			}
		}
	}
	return matches, nil
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
