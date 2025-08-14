package kafka

type TxEvent struct {
	UserID      string `json:"user_id"`
	Address     string `json:"address"`
	Direction   string `json:"direction"`
	TxHash      string `json:"tx_hash"`
	BlockNumber uint64 `json:"block_number"`
	BlockTime   int64  `json:"block_time"`
	From        string `json:"from"`
	To          string `json:"to"`
	AmountWei   string `json:"amount_wei"`
	FeeWei      string `json:"fee_wei"`
	Status      string `json:"status"`
	ChainID     string `json:"chain_id"`
}
