package processor

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/ARK21/deblock/internal/app/filter"
	"github.com/ARK21/deblock/internal/app/kafka"
	"github.com/ARK21/deblock/internal/app/rpc"
	"github.com/stretchr/testify/require"
)

func TestWeiToEth(t *testing.T) {
	wei := new(big.Int)
	wei.SetString("1234500000000000000", 10) // 1.2345 ETH
	got := weiToEth(wei)
	if got != "1.234500000000000000" {
		t.Fatalf("want 1.2345..., got %s", got)
	}
}

// ---- capture bus to collect events ----
type captureBus struct{ out []kafka.MatchedTxEvent }

var _ kafka.Publisher = (*captureBus)(nil)

func (c *captureBus) Publish(_ context.Context, event any) error {
	switch e := event.(type) {
	case kafka.MatchedTxEvent:
		c.out = append(c.out, e)
	case *kafka.MatchedTxEvent:
		c.out = append(c.out, *e)
	default:
		return fmt.Errorf("unexpected event type %T", event)
	}
	return nil
}

// ---- mock RPC that returns receipts deterministically ----
type mockRPC struct {
	rc map[string]rpc.Receipt
}

func (m *mockRPC) SubscribeNewHeads(context.Context) (<-chan rpc.Header, <-chan error) {
	return nil, nil
}
func (m *mockRPC) GetBlockByHash(context.Context, string, bool) (rpc.Block, error) {
	return rpc.Block{}, nil
}
func (m *mockRPC) GetBlockByNumber(context.Context, uint64, bool) (rpc.Block, error) {
	return rpc.Block{}, nil
}
func (m *mockRPC) GetTxReceipt(_ context.Context, h string) (rpc.Receipt, error) { return m.rc[h], nil }
func (m *mockRPC) BatchGetReceipts(_ context.Context, hashes []string) (map[string]rpc.Receipt, error) {
	out := make(map[string]rpc.Receipt, len(hashes))
	for _, h := range hashes {
		out[h] = m.rc[h]
	}
	return out, nil
}
func (m *mockRPC) GetChainID(context.Context) (uint64, error)     { return 1, nil }
func (m *mockRPC) GetBlockNumber(context.Context) (uint64, error) { return 0, nil }

func TestProcessBlock_EmitsInAndOut_WithFees(t *testing.T) {
	ctx := context.Background()

	// watch 2 addresses
	addrA := "0x0000000000000000000000000000000000000AaA" // user uA
	addrB := "0x0000000000000000000000000000000000000BbB" // user uB
	matcher := filter.NewMatcher(map[string]string{
		addrA: "uA",
		addrB: "uB",
	})

	// block with two txs: A->B (value>0), B->X (value 0)
	blk := rpc.Block{
		Number:     123,
		Hash:       "H123",
		ParentHash: "H122",
		Timestamp:  1710000000,
		Txs: []rpc.Tx{
			{Hash: "0xTX1", From: addrA, To: &[]string{addrB}[0], Value: "2100000000000000"}, // 0.0021 ETH
			{Hash: "0xTX2", From: addrB, To: &[]string{"0x0000000000000000000000000000000000000cCc"}[0], Value: "0"},
		},
	}
	// receipts: gasUsed=21000, price=2 gwei (fee=42000 gwei = 42000*1e9 wei)
	rc := map[string]rpc.Receipt{
		"0xTX1": {Status: 1, GasUsed: "21000", EffectiveGasPrice: "2000000000"}, // 2 gwei
		"0xTX2": {Status: 1, GasUsed: "42000", EffectiveGasPrice: "1000000000"}, // 1 gwei
	}
	rpcMock := &mockRPC{rc: rc}

	bus := &captureBus{}
	s := &Service{RPC: rpcMock, Matcher: matcher, EventBus: bus, ChainID: 1}

	n, err := s.ProcessBlock(ctx, blk, false)
	require.NoError(t, err)
	require.Equal(t, 2, n)     // two matches (A and B addresses)
	require.Len(t, bus.out, 3) // A->B produces 2 events (in/out), B->X produces 1 (out) = 3

	// Check one incoming and one outgoing of TX1
	var in, out kafka.MatchedTxEvent
	for _, e := range bus.out {
		if e.TxHash == "0xTX1" && e.Direction == "in" {
			in = e
		}
		if e.TxHash == "0xTX1" && e.Direction == "out" {
			out = e
		}
	}

	require.Equal(t, "uB", in.UserID)
	require.Equal(t, "in", in.Direction)
	require.Equal(t, "0.002100000000000000", in.AmountEth)
	require.Equal(t, "0", in.FeeWei) // receiver pays no fee

	require.Equal(t, "uA", out.UserID)
	require.Equal(t, "out", out.Direction)
	require.Equal(t, "0.002100000000000000", out.AmountEth)
	// fee = 21000 * 2e9 = 42000000000000 wei
	require.Equal(t, "42000000000000", out.FeeWei)
	require.Equal(t, "0.000042000000000000", out.FeeEth)

	// reorg flag remains false here
	for _, e := range bus.out {
		require.False(t, e.Reorged)
	}
}
