package kafka

import (
	"time"

	"github.com/google/uuid"
)

type MessageHeader struct {
	ID            string `json:"id"`
	EventName     string `json:"event_name"`
	CorrelationID string `json:"correlation_id"` // TODO delete?
	PublishedAt   string `json:"published_at"`
}

func NewMessageHeader(eventName string) MessageHeader {
	return MessageHeader{
		ID:          uuid.NewString(),
		EventName:   eventName,
		PublishedAt: time.Now().Format(time.RFC3339),
	}
}

type MatchedTxEvent struct {
	Header MessageHeader `json:"header"`

	UserID      string `json:"user_id"`
	Address     string `json:"address"`
	Direction   string `json:"direction"`
	TxHash      string `json:"tx_hash"`
	BlockNumber uint64 `json:"block_number"`
	BlockTime   int64  `json:"block_time"`
	From        string `json:"from"`
	To          string `json:"to"`

	AmountWei string `json:"amount_wei"`
	AmountEth string `json:"amount_eth"`
	FeeWei    string `json:"fee_wei"`
	FeeEth    string `json:"fee_eth"`

	Status  string `json:"status"`
	ChainID uint64 `json:"chain_id"`
	Reorged bool   `json:"reorged"`
}
