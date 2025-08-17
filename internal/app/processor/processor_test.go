package processor

import (
	"math/big"
	"testing"
)

func TestWeiToEth(t *testing.T) {
	wei := new(big.Int)
	wei.SetString("1234500000000000000", 10) // 1.2345 ETH
	got := weiToEth(wei)
	if got != "1.234500000000000000" {
		t.Fatalf("want 1.2345..., got %s", got)
	}
}
