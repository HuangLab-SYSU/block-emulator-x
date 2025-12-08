package partition

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	edgesNum      = 30000
	vertexNum     = 100
	w             = 0.5
	maxIterations = 100
	shardNum      = 4
)

func TestCLPA(t *testing.T) {
	edges := generateRandomEdges()
	c := NewCLPAState(w, maxIterations, shardNum)
	for _, e := range edges {
		c.AddEdge(e[0], e[1])
	}

	result, _ := c.CLPAPartition()
	for k, v := range result {
		require.NotEqual(t, DefaultAccountLoc(k, shardNum), int64(v))
	}
}

func generateRandomEdges() [][]Vertex {
	vertexes := make([]Vertex, vertexNum)
	for i := range vertexes {
		var addr [20]byte
		_, _ = rand.Read(addr[:])
		vertexes[i] = Vertex{Addr: addr}
	}

	ret := make([][]Vertex, edgesNum)
	for i := 0; i < edgesNum; i++ {
		a, _ := rand.Int(rand.Reader, big.NewInt(int64(vertexNum)))
		b, _ := rand.Int(rand.Reader, big.NewInt(int64(vertexNum)))
		if a.Int64() == b.Int64() {
			i-- // avoid a self-loop tx
			continue
		}
		ret[i] = []Vertex{vertexes[a.Int64()], vertexes[b.Int64()]}
	}
	return ret
}
