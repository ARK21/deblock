package filter

import "github.com/ethereum/go-ethereum/common"

type Matcher struct {
	users map[common.Address]string
}

func NewMatcher(addrs map[string]string) *Matcher {
	m := &Matcher{
		users: make(map[common.Address]string, len(addrs)),
	}
	for a, uid := range addrs {
		addr := common.HexToAddress(a)
		m.users[addr] = uid
	}

	return m
}

func (m *Matcher) Match(from string, to *string) (string, string, bool) {
	var fromUID, toUID string
	if from != "" {
		if iud, ok := m.users[common.HexToAddress(from)]; ok {
			fromUID = iud
		}
	}
	if to != nil {
		if iud, ok := m.users[common.HexToAddress(*to)]; ok {
			toUID = iud
		}
	}
	return fromUID, toUID, (fromUID != "" || toUID != "")
}
