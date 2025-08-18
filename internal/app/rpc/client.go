package rpc

import "context"

type Header struct {
	Hash, ParentHash string
	Number           uint64
}

type Client interface {
	SubscribeNewHeads(ctx context.Context) (<-chan Header, <-chan error)
	GetBlockByHash(ctx context.Context, hash string, fullTx bool) (Block, error)
	GetBlockByNumber(ctx context.Context, number uint64, fullTx bool) (Block, error)
	GetTxReceipt(ctx context.Context, txHash string) (Receipt, error)
	GetChainID(ctx context.Context) (uint64, error)
	GetBlockNumber(ctx context.Context) (uint64, error)
	BatchGetReceipts(ctx context.Context, hashes []string) (map[string]Receipt, error)
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
