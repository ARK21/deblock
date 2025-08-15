package heads

type Header struct {
	Hash, ParentHash string
	Number           uint64
}

// Finalizer buffers heads and emits them in order once they're N confirmations deep.
type Finalizer struct {
	confs   uint64
	latest  uint64
	next    uint64
	pending map[uint64]Header
}

func NewFinalizer(confs int) *Finalizer {
	return &Finalizer{
		confs:   uint64(confs),
		pending: make(map[uint64]Header),
	}
}

func (f *Finalizer) Add(h Header) []Header {
	f.pending[h.Number] = h
	if h.Number > f.latest {
		f.latest = h.Number
	}
	threshold := uint64(0)
	if f.latest >= f.confs {
		threshold = f.latest - f.confs
	}
	if f.next == 0 {
		f.next = h.Number
	}

	finalized := make([]Header, 0)

	for f.next != 0 && f.next <= threshold {
		if hd, ok := f.pending[f.next]; ok {
			finalized = append(finalized, hd)
			delete(f.pending, f.next)
		}
		f.next++
	}
	return finalized
}
