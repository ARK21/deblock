package reorg

import (
	"context"
	"testing"

	"github.com/ARK21/deblock/internal/app/rpc"
	"github.com/stretchr/testify/require"
)

type mockRPC struct {
	byHash   map[string]rpc.Block
	byNumber map[uint64]rpc.Block
}

func (m *mockRPC) SubscribeNewHeads(context.Context) (<-chan rpc.Header, <-chan error) {
	return nil, nil
}
func (m *mockRPC) GetBlockByHash(_ context.Context, h string, _ bool) (rpc.Block, error) {
	return m.byHash[h], nil
}
func (m *mockRPC) GetBlockByNumber(_ context.Context, n uint64, _ bool) (rpc.Block, error) {
	return m.byNumber[n], nil
}
func (m *mockRPC) GetTxReceipt(context.Context, string) (rpc.Receipt, error) {
	return rpc.Receipt{}, nil
}
func (m *mockRPC) BatchGetReceipts(context.Context, []string) (map[string]rpc.Receipt, error) {
	return nil, nil
}
func (m *mockRPC) GetChainID(context.Context) (uint64, error)     { return 1, nil }
func (m *mockRPC) GetBlockNumber(context.Context) (uint64, error) { return 0, nil }

func TestManager_BasicAndReorg(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager(12)
	m := &mockRPC{byHash: map[string]rpc.Block{}, byNumber: map[uint64]rpc.Block{}}
	put := func(b rpc.Block) { m.byHash[b.Hash] = b; m.byNumber[b.Number] = b }

	// Canonical: 100(H100)->101(H101)->102(H102)
	put(rpc.Block{Number: 100, Hash: "H100", ParentHash: "H099"})
	put(rpc.Block{Number: 101, Hash: "H101", ParentHash: "H100"})
	put(rpc.Block{Number: 102, Hash: "H102", ParentHash: "H101"})

	// Process 100 and 101
	require.True(t, mgr.ParentOK(m.byNumber[100]))
	mgr.Record(m.byNumber[100])
	require.True(t, mgr.ParentOK(m.byNumber[101]))
	mgr.Record(m.byNumber[101])

	// Fork at 101': 101'(H101p, parent H100) -> 102'(H102p, parent H101p)
	put(rpc.Block{Number: 101, Hash: "H101p", ParentHash: "H100"})
	put(rpc.Block{Number: 102, Hash: "H102p", ParentHash: "H101p"})

	// Parent mismatch on 102'
	require.False(t, mgr.ParentOK(m.byHash["H102p"]))

	ancN, ancH, ok := mgr.CommonAncestor(ctx, m, "H102p", 102)
	require.True(t, ok)
	require.Equal(t, uint64(100), ancN)
	require.Equal(t, "H100", ancH)

	from, to, err := mgr.RewindRange(ancN)
	require.NoError(t, err)
	require.Equal(t, uint64(101), from)
	require.Equal(t, uint64(101), to)

	mgr.ResetAbove(ancN)
	require.Equal(t, ancN, mgr.Highest())
}

func TestParentOK(t *testing.T) {
	m := NewManager(12)
	// first block: allow (no history)
	b100 := rpc.Block{Number: 100, Hash: "0xH100", ParentHash: "0xH099"}
	if !m.ParentOK(b100) {
		t.Fatal("first block should be OK")
	}
	m.Record(b100)

	// correct parent
	b101 := rpc.Block{Number: 101, Hash: "0xH101", ParentHash: "0xH100"}
	if !m.ParentOK(b101) {
		t.Fatal("should be OK when parent matches")
	}
	m.Record(b101)

	// wrong parent
	bad := rpc.Block{Number: 102, Hash: "0xBAD", ParentHash: "0xNOTH101"}
	if m.ParentOK(bad) {
		t.Fatal("should detect mismatch")
	}
}
