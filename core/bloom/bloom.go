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

// bfHashFunc is the wrapped hash function for bloom filter.
type bfHashFunc func([]byte) []byte

type Filter struct {
	b        *bitset.BitSet
	bitLen   uint
	bfHashFs []bfHashFunc
}

func NewFilter(cfg config.BloomFilterCfg) (*Filter, error) {
	if cfg.BitsetLen > maxBitsetLen {
		return nil, fmt.Errorf("length of Bitset must be <= %d", maxBitsetLen)
	}
	return &Filter{
		bitLen:   uint(cfg.BitsetLen),
		b:        bitset.New(uint(cfg.BitsetLen)),
		bfHashFs: getFilterHashFs(cfg.FilterHashFunc),
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

func getFilterHashFs(hashFuncStrList []string) []bfHashFunc {
	ret := make([]bfHashFunc, len(hashFuncStrList))
	for i, hashFuncStr := range hashFuncStrList {
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
