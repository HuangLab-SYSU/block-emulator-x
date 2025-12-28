package bloom

import (
	"encoding/binary"
	"fmt"

	"github.com/bits-and-blooms/bitset"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
)

const (
	hashByteLen  = 8
	maxBitsetLen = 1 << 24
)

// filterFuncRegistry is the registry table of Bloom Filter functions.
var filterFuncRegistry = map[string]FilterHashFunc{
	"sha256": Sha256,
	"sha512": Sha512,
	"sha1":   Sha1,
}

// FilterHashFunc is the wrapped hash function for bloom filter.
type FilterHashFunc func([]byte) []byte

type Filter struct {
	B            bitset.BitSet
	FilterHashFs []string
}

func NewFilter(cfg config.BloomFilterCfg) (*Filter, error) {
	if cfg.BitsetLen > maxBitsetLen {
		return nil, fmt.Errorf("length of Bitset must be <= %d", maxBitsetLen)
	}

	return &Filter{
		B:            *bitset.New(uint(cfg.BitsetLen)),
		FilterHashFs: cfg.FilterHashFunc,
	}, nil
}

func (f *Filter) Add(elements ...[]byte) {
	filterFunc := f.getFilterHashFs()
	for _, element := range elements {
		for _, h := range filterFunc {
			f.B.Set(byte2uint(h(element)) % f.B.Len())
		}
	}
}

func (f *Filter) Contains(hash []byte) bool {
	filterFunc := f.getFilterHashFs()
	for _, h := range filterFunc {
		if !f.B.Test(byte2uint(h(hash)) % f.B.Len()) {
			return false
		}
	}

	return true
}

func (f *Filter) Equal(other Filter) bool {
	return f.B.Equal(&other.B)
}

func (f *Filter) getFilterHashFs() []FilterHashFunc {
	ret := make([]FilterHashFunc, len(f.FilterHashFs))

	for i, hashFuncStr := range f.FilterHashFs {
		if hashFunc, ok := filterFuncRegistry[hashFuncStr]; ok {
			ret[i] = hashFunc
		} else {
			return defaultFilterFs
		}
	}

	return ret
}

func byte2uint(b []byte) uint {
	if len(b) < hashByteLen {
		// pad to hashByteLen bytes
		b = append(make([]byte, hashByteLen-len(b)), b...)
	}

	return uint(binary.BigEndian.Uint64(b[len(b)-hashByteLen:]))
}
