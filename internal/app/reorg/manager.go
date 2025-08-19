package reorg

import (
	"context"
	"fmt"

	"github.com/ARK21/deblock/internal/app/rpc"
)

type Manager struct {
	depth uint64
	// last processed canonical hashes by number (only keep a sliding window)
	byNum   map[uint64]string
	highest uint64
}

func NewManager(depth int) *Manager {
	if depth < 1 {
		depth = 12
	}
	return &Manager{
		depth:   uint64(depth),
		byNum:   make(map[uint64]string, depth+2),
		highest: 0,
	}
}

// ParentOK returns true if blk.ParentHash matches our recorded hash at (blk.Number-1).
// If we have no history (first run), it's considered OK.
func (m *Manager) ParentOK(blk rpc.Block) bool {
	if m.highest == 0 {
		return true
	}
	prevHash, ok := m.byNum[blk.Number-1]
	if !ok {
		return true
	} // no info; allow and record
	return prevHash == blk.ParentHash
}

// Record stores blk as processed canonical block and prunes old history.
func (m *Manager) Record(blk rpc.Block) {
	m.byNum[blk.Number] = blk.Hash
	if blk.Number > m.highest {
		m.highest = blk.Number
	}
	// prune older than window
	lower := uint64(0)
	if m.highest > m.depth {
		lower = m.highest - m.depth
	}
	for n := range m.byNum {
		if n < lower {
			delete(m.byNum, n)
		}
	}
}

// CommonAncestor walks up the *new* chain from head (by following parent hashes)
// until it finds a block number within our window whose hash equals what we recorded.
// Returns (number, hash, found).
func (m *Manager) CommonAncestor(
	ctx context.Context,
	c rpc.Client,
	headHash string,
	headNum uint64,
) (uint64, string, bool) {
	// fast path: if we have entry for headNum and hashes equal → ancestor is head itself
	if h, ok := m.byNum[headNum]; ok && h == headHash {
		return headNum, headHash, true
	}

	curHash := headHash
	curNum := headNum
	steps := uint64(0)
	for steps <= m.depth && curNum > 0 {
		// Do we have a recorded hash for curNum and does it match?
		if rec, ok := m.byNum[curNum]; ok && rec == curHash {
			return curNum, curHash, true
		}
		// Move to parent
		blk, err := c.GetBlockByHash(ctx, curHash, false)
		if err != nil {
			return 0, "", false
		}
		curHash = blk.ParentHash
		curNum = blk.Number - 1
		steps++
	}
	return 0, "", false
}

// RewindRange returns the (start, end) numbers to reprocess after finding ancestor.
// All blocks in (ancestor+1 .. oldHighest] must be considered stale and re-emitted.
func (m *Manager) RewindRange(ancestor uint64) (uint64, uint64, error) {
	if m.highest <= ancestor {
		return 0, 0, fmt.Errorf("nothing to rewind (highest=%d, ancestor=%d)", m.highest, ancestor)
	}
	return ancestor + 1, m.highest, nil
}

func (m *Manager) ResetAbove(ancestor uint64) {
	for n := range m.byNum {
		if n > ancestor {
			delete(m.byNum, n)
		}
	}
	m.highest = ancestor
}
