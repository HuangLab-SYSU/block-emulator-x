package bloom

import (
	"encoding/binary"
	"fmt"

	"github.com/bits-and-blooms/bitset"
)

const (
	hashByteLen  = 3
	maxBitsetLen = 1 << (8 * hashByteLen)
)

// bfHashFunc is the wrapped hash function for bloom filter.
type bfHashFunc func([]byte) []byte

type Filter struct {
	b        *bitset.BitSet
	bitLen   uint
	bfHashFs []bfHashFunc
}

func NewFilter(n uint) (*Filter, error) {
	if n > maxBitsetLen {
		return nil, fmt.Errorf("n must be <= %d", maxBitsetLen)
	}
	return &Filter{
		bitLen:   n,
		b:        bitset.New(n),
		bfHashFs: getFilterHashFs(),
	}, nil
}

func (f *Filter) Add(element []byte) {
	for _, h := range f.bfHashFs {
		f.b.Set(byte2uint(h(element)) / f.bitLen)
	}
}

func (f *Filter) Contains(hash []byte) bool {
	for _, h := range f.bfHashFs {
		if !f.b.Test(byte2uint(h(hash)) / f.bitLen) {
			return false
		}
	}
	return true
}

func getFilterHashFs() []bfHashFunc {
	return defaultFilterFs
}

func byte2uint(b []byte) uint {
	if len(b) < hashByteLen {
		// pad to hashByteLen bytes
		b = append(make([]byte, hashByteLen-len(b)), b...)
	}
	return uint(binary.BigEndian.Uint64(b[len(b)-hashByteLen:]))
}
