package filter

import "testing"

func TestMatch(t *testing.T) {
	m := NewMatcher(map[string]string{
		"0x000000000000000000000000000000000000dEaD": " u1",
	})
	fromUID, toUID, ok := m.Match("0x000000000000000000000000000000000000dEaD", nil)
	if !ok {
		t.Fatal("expected match to succeed")
	}
	t.Logf("fromUID: %s, toUID: %s", fromUID, toUID)
}
