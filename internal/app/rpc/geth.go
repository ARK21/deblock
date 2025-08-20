package rpc

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ARK21/deblock/internal/app/metrics"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

type GethClient struct {
	ws   *rpc.Client
	http *rpc.Client
}

func NewGethClient(ctx context.Context, wsURL, httpURL string) (*GethClient, error) {
	var ws, http *rpc.Client
	var err error
	if wsURL != "" {
		if ws, err = rpc.DialContext(ctx, wsURL); err != nil {
			return nil, fmt.Errorf("ws dial error: %w", err)
		}
	}
	if httpURL != "" {
		if http, err = rpc.DialContext(ctx, httpURL); err != nil {
			return nil, fmt.Errorf("http dial error: %w", err)
		}
	}
	if ws == nil {
		return nil, fmt.Errorf("ws client is nil")
	}
	if http == nil {
		return nil, fmt.Errorf("http client is nil")
	}

	return &GethClient{ws: ws, http: http}, nil
}

type rpcHead struct {
	Hash       common.Hash    `json:"hash"`
	ParentHash common.Hash    `json:"parentHash"`
	Number     hexutil.Uint64 `json:"number"`
}

func (c *GethClient) SubscribeNewHeads(ctx context.Context) (<-chan Header, <-chan error) {
	out := make(chan Header, 64)
	errs := make(chan error, 1)
	if c.ws == nil {
		errs <- fmt.Errorf("ws client is nil")
		close(out)
		close(errs)
		return out, errs
	}

	heads := make(chan rpcHead, 64)
	sub, err := c.ws.Subscribe(ctx, "eth", heads, "newHeads")
	if err != nil {
		errs <- fmt.Errorf("subscribe: %w", err)
		close(out)
		close(errs)
		return out, errs
	}
	go func() {
		defer close(out)
		defer close(errs)
		for {
			select {
			case h := <-heads:
				out <- Header{Hash: h.Hash.Hex(), ParentHash: h.ParentHash.Hex(), Number: uint64(h.Number)}
			case err := <-sub.Err():
				errs <- err
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, errs
}

type rpcTx struct {
	Hash  common.Hash     `json:"hash"`
	From  common.Address  `json:"from"`
	To    *common.Address `json:"to"`
	Value *hexutil.Big    `json:"value"`
}

type rpcBlock struct {
	Hash       common.Hash    `json:"hash"`
	Number     hexutil.Uint64 `json:"number"`
	ParentHash common.Hash    `json:"parentHash"`
	Timestamp  hexutil.Uint64 `json:"timestamp"`
	Txs        []rpcTx        `json:"transactions"`
}

func (c *GethClient) GetBlockByHash(ctx context.Context, hash string, fullTx bool) (Block, error) {
	var rb rpcBlock
	err := c.http.CallContext(ctx, &rb, "eth_getBlockByHash", hash, fullTx)
	metrics.RPCCall("eth_getBlockByHash", err == nil)
	if err != nil {
		return Block{}, err
	}
	return convertBlock(rb), nil
}

func (c *GethClient) GetBlockByNumber(ctx context.Context, number uint64, fullTx bool) (Block, error) {
	var rb rpcBlock
	err := c.http.CallContext(ctx, &rb, "eth_getBlockByNumber", hexutil.Uint64(number), fullTx)
	metrics.RPCCall("eth_getBlockByNumber", err == nil)
	if err != nil {
		return Block{}, err
	}
	return convertBlock(rb), nil
}

type rpcReceipt struct {
	Status            hexutil.Uint64 `json:"status"`
	GasUsed           hexutil.Uint64 `json:"gasUsed"`
	EffectiveGasPrice *hexutil.Big   `json:"effectiveGasPrice"`
}

func (c *GethClient) GetTxReceipt(ctx context.Context, txHash string) (Receipt, error) {
	var rr rpcReceipt
	err := c.http.CallContext(ctx, &rr, "eth_getTransactionReceipt", txHash)
	metrics.RPCCall("eth_getTransactionReceipt", err == nil)
	if err != nil {
		return Receipt{}, err
	}
	egp := big.NewInt(0)
	if rr.EffectiveGasPrice != nil {
		egp = (*big.Int)(rr.EffectiveGasPrice)
	}
	return Receipt{
		Status:            uint64(rr.Status),
		GasUsed:           fmt.Sprintf("%d", uint64(rr.GasUsed)),
		EffectiveGasPrice: egp.String(),
	}, nil
}

func (c *GethClient) BatchGetReceipts(ctx context.Context, hashes []string) (map[string]Receipt, error) {
	out := make(map[string]Receipt, len(hashes))
	if len(hashes) == 0 {
		return out, nil
	}

	const chunk = 50 // safe default for most providers

	for i := 0; i < len(hashes); i += chunk {
		j := i + chunk
		if j > len(hashes) {
			j = len(hashes)
		}
		h := hashes[i:j]
		rr := make([]rpcReceipt, len(h))
		batch := make([]rpc.BatchElem, len(h))
		for k, hash := range h {
			batch[k] = rpc.BatchElem{
				Method: "eth_getTransactionReceipt",
				Args:   []interface{}{hash},
				Result: &rr[k],
			}
		}
		metrics.ObserveReceiptBatch(len(h))
		metrics.ReceiptsInFlight(len(h))
		err := c.http.BatchCallContext(ctx, batch)
		metrics.ReceiptsInFlight(0)
		metrics.RPCCall("eth_getTransactionReceipt_batch", err == nil)
		if err != nil {
			return nil, fmt.Errorf("batch call error: %w", err)
		}
		for k, r := range rr {
			egp := big.NewInt(0)
			if r.EffectiveGasPrice != nil {
				egp = (*big.Int)(r.EffectiveGasPrice)
			}
			out[hashes[k]] = Receipt{
				Status:            uint64(r.Status),
				GasUsed:           fmt.Sprintf("%d", uint64(r.GasUsed)),
				EffectiveGasPrice: egp.String(),
			}
		}
	}

	return out, nil
}

func (c *GethClient) GetChainID(ctx context.Context) (uint64, error) {
	var hex hexutil.Big
	if err := c.http.CallContext(ctx, &hex, "eth_chainId"); err != nil {
		return 0, err
	}

	return (*big.Int)(&hex).Uint64(), nil
}

func (c *GethClient) GetBlockNumber(ctx context.Context) (uint64, error) {
	var num hexutil.Uint64
	err := c.http.CallContext(ctx, &num, "eth_blockNumber")
	metrics.RPCCall("eth_blockNumber", err == nil)
	if err != nil {
		return 0, err
	}
	return uint64(num), nil
}

func convertBlock(rb rpcBlock) Block {
	b := Block{
		Hash:       rb.Hash.Hex(),
		Number:     uint64(rb.Number),
		ParentHash: rb.ParentHash.Hex(),
		Timestamp:  uint64(rb.Timestamp),
		Txs:        make([]Tx, len(rb.Txs)),
	}
	for _, t := range rb.Txs {
		var to *string
		if t.To != nil {
			s := t.To.Hex()
			to = &s
		}
		val := big.NewInt(0)
		if t.Value != nil {
			val = (*big.Int)(t.Value)
		}
		b.Txs = append(b.Txs, Tx{
			Hash:  t.Hash.Hex(),
			From:  t.From.Hex(),
			To:    to,
			Value: val.String(),
		})
	}

	return b
}
