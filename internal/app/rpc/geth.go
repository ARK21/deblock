package rpc

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

type Header struct {
	Hash, ParentHash string
	Number           uint64
}

type Client interface {
	SubscribeNewHeads(ctx context.Context) (<-chan Header, <-chan error)
	GetBlockByHash(ctx context.Context, hash string, fullTx bool) (Block, error)
	GetBlockByNumber(ctx context.Context, number uint64, fullTx bool) (Block, error)
	GetTxReceipt(ctx context.Context, txHash string) (Receipt, error)
}

type Tx struct {
	Hash, From string
	To         *string
	Value      string
}

type Block struct {
	Number     uint64
	ParentHash string
	Timestamp  uint64
	Txs        []Tx
}

type Receipt struct {
	Status            uint64
	GasUsed           string
	EffectiveGasPrice string
}

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
	if ws == nil && http == nil {
		return nil, fmt.Errorf("both ws and http clients are nil")
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
	Number     hexutil.Uint64 `json:"number"`
	ParentHash common.Hash    `json:"parentHash"`
	Timestamp  hexutil.Uint64 `json:"timestamp"`
	Txs        []rpcTx        `json:"transactions"`
}

func (c *GethClient) GetBlockByHash(ctx context.Context, hash string, fullTx bool) (Block, error) {
	if c.http == nil {
		return Block{}, fmt.Errorf("http client is nil")
	}
	var rb rpcBlock
	if err := c.http.CallContext(ctx, &rb, "eth_getBlockByHash", hash, fullTx); err != nil {
		return Block{}, err
	}
	return convertBlock(rb), nil
}

func (c *GethClient) GetBlockByNumber(ctx context.Context, number uint64, fullTx bool) (Block, error) {
	if c.http == nil {
		return Block{}, fmt.Errorf("http client is nil")
	}
	var rb rpcBlock
	if err := c.http.CallContext(ctx, &rb, "eth_getBlockByNumber", hexutil.Uint64(number), fullTx); err != nil {
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
	if c.http == nil {
		return Receipt{}, fmt.Errorf("http client is nil")
	}
	var rr rpcReceipt
	if err := c.http.CallContext(ctx, &rr, "eth_getTransactionReceipt", txHash); err != nil {
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

func convertBlock(rb rpcBlock) Block {
	b := Block{
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
