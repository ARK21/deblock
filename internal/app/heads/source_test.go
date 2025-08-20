package heads

import (
	"context"
	"testing"
	"time"

	"github.com/ARK21/deblock/internal/app/rpc"
	"github.com/stretchr/testify/require"
)

// fake client: WS fails; BlockNumber increments
type fakeClient struct{ n uint64 }

func (f *fakeClient) SubscribeNewHeads(context.Context) (<-chan rpc.Header, <-chan error) {
	return nil, make(chan error)
}
func (f *fakeClient) GetBlockByHash(context.Context, string, bool) (rpc.Block, error) {
	return rpc.Block{}, nil
}
func (f *fakeClient) GetBlockByNumber(context.Context, uint64, bool) (rpc.Block, error) {
	return rpc.Block{}, nil
}
func (f *fakeClient) GetTxReceipt(context.Context, string) (rpc.Receipt, error) {
	return rpc.Receipt{}, nil
}
func (f *fakeClient) BatchGetReceipts(context.Context, []string) (map[string]rpc.Receipt, error) {
	return nil, nil
}
func (f *fakeClient) GetChainID(context.Context) (uint64, error) { return 1, nil }
func (f *fakeClient) GetBlockNumber(context.Context) (uint64, error) {
	f.n++
	return f.n, nil
}

func TestSource_FallbackToPolling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	src := NewSource(&fakeClient{}, 10*time.Millisecond, 100*time.Millisecond, 500*time.Millisecond)

	hch, _ := src.Run(ctx)

	var got []uint64
	for {
		select {
		case h, ok := <-hch:
			if !ok {
				goto END
			}
			got = append(got, h.Number)
			if len(got) >= 3 {
				goto END
			}
		case <-ctx.Done():
			goto END
		}
	}
END:
	require.GreaterOrEqual(t, len(got), 2, "expected some heads via polling")
	require.Equal(t, got[0]+1, got[1], "monotonic increasing heads")
}
