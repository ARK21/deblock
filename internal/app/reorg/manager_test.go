package reorg

import (
	"testing"

	"github.com/ARK21/deblock/internal/app/rpc"
)

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
