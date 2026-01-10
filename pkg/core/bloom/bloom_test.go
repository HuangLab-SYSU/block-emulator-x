package bloom

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
)

const (
	bsLen = 1 << 10
	s1    = "hello world"
	s2    = "hello new world"
)

func TestBloom(t *testing.T) {
	f, err := NewFilter(config.BloomFilterCfg{BitsetLen: bsLen, FilterHashFunc: []string{"sha256", "sha512", "sha1"}})
	require.NoError(t, err)

	f.Add([]byte(s1), []byte(s2))

	// Contains
	require.True(t, f.Contains([]byte(s1)))
	require.True(t, f.Contains([]byte(s2)))

	// Equal
	fByte, err := json.Marshal(f)
	require.NoError(t, err)
	var otherF Filter
	err = json.Unmarshal(fByte, &otherF)
	require.NoError(t, err)
	require.True(t, otherF.Equal(*f))
}
