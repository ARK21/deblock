package filter

import "github.com/ethereum/go-ethereum/common"

type Matcher struct {
	users map[[20]byte]string
}

func NewMatcher(addrs map[string]string) *Matcher {
	m := &Matcher{
		users: make(map[[20]byte]string, len(addrs)),
	}
	for a, uid := range addrs {
		addr := common.HexToAddress(a)
		m.users[addr] = uid
	}

	return m
}

func (m *Matcher) Match(from string, to *string) (fromUID, toUID string, ok bool) {
	var f, t string
	if from != "" {
		f = common.HexToAddress(from).Hex()

	}
	if to != nil {
		t = common.HexToAddress(*to).Hex()
	}

	if uid, okf := m.users[common.HexToAddress(f)]; okf {
		return uid, "", true
	}
	if to != nil {
		if uid, okt := m.users[common.HexToAddress(t)]; okt {
			return "", uid, true
		}
	}

	return "", "", false
}
