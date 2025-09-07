package bloom

import (
	"encoding/binary"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/bits-and-blooms/bitset"
)

const (
	hashByteLen  = 3
	maxBitsetLen = 1 << (8 * hashByteLen)
)

// FilterHashFunc is the wrapped hash function for bloom filter.
type FilterHashFunc func([]byte) []byte

type Filter struct {
	B            *bitset.BitSet
	FilterHashFs []string
}

func NewFilter(cfg config.BloomFilterCfg) (*Filter, error) {
	if cfg.BitsetLen > maxBitsetLen {
		return nil, fmt.Errorf("length of Bitset must be <= %d", maxBitsetLen)
	}
	return &Filter{
		B:            bitset.New(uint(cfg.BitsetLen)),
		FilterHashFs: cfg.FilterHashFunc,
	}, nil
}

func (f *Filter) Add(elements ...[]byte) {
	filterFunc := f.getFilterHashFs()
	for _, element := range elements {
		for _, h := range filterFunc {
			f.B.Set(byte2uint(h(element)) / f.B.Len())
		}
	}
}

func (f *Filter) Contains(hash []byte) bool {
	filterFunc := f.getFilterHashFs()
	for _, h := range filterFunc {
		if !f.B.Test(byte2uint(h(hash)) / f.B.Len()) {
			return false
		}
	}
	return true
}

func (f *Filter) getFilterHashFs() []FilterHashFunc {
	ret := make([]FilterHashFunc, len(f.FilterHashFs))
	for i, hashFuncStr := range f.FilterHashFs {
		switch hashFuncStr {
		case "sha256":
			ret[i] = Sha256
		case "sha512":
			ret[i] = Sha512
		case "sha1":
			ret[i] = Sha1
		default:
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
