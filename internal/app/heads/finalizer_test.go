package heads

import "testing"

func TestFinalizer_OrderAndConfs(t *testing.T) {
	f := NewFinalizer(3)
	var out []Header

	for _, n := range []uint64{10, 11, 12, 13, 14} {
		fs := f.Add(Header{Number: n})
		out = append(out, fs...)
	}
	// With confs=3 we finalize 10 and 11 when we see 13 & 14.
	if len(out) != 2 || out[0].Number != 10 || out[1].Number != 11 {
		t.Fatalf("unexpected finalized: %#v", out)
	}
}
