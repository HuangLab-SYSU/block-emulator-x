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

// TestCLPAOwnShardScoring checks that the vertex's own shard takes part in the scoring: when most
// neighbours are inside the shard, the vertex stays put.
//
// Graph: A sits in shard 0 with 6 neighbours inside it and one neighbour each in shard 1, 2 and 3;
// each of the three out-of-shard shards holds a 6-vertex ring. Then W = (9, 7, 7, 7), and A scores
//
//	shard 0:      6/9 · (1 − 0.5·9/7) = 0.2381
//	shard 1, 2, 3: 1/9 · (1 − 0.5·7/7) = 0.0556
//
// The own shard scores highest, so A must stay. An implementation that excludes the own shard never
// sees 0.2381, can only pick one of the three out-of-shard shards, and always moves A.
func TestCLPAOwnShardScoring(t *testing.T) {
	sa := Vertex{Addr: testAddr(100)} // 100 mod 4 = 0
	own := make([]Vertex, 6)

	for j := range own {
		own[j] = Vertex{Addr: testAddr(4 * (j + 1))} // 4, 8, …, 24, all in shard 0
	}

	ring := func(res int) []Vertex {
		out := make([]Vertex, 6)
		for j := range out {
			out[j] = Vertex{Addr: testAddr(4*(j+1) + res)}
		}
		return out
	}
	sp, sq, sr := ring(1), ring(2), ring(3)

	c := NewCLPAState(w, maxIterations, shardNum)
	c.AddEdge(sa, sp[0])
	c.AddEdge(sa, sq[0])
	c.AddEdge(sa, sr[0])
	for _, v := range own {
		c.AddEdge(sa, v)
	}
	for _, rg := range [][]Vertex{sp, sq, sr} {
		for j := range rg {
			c.AddEdge(rg[j], rg[(j+1)%len(rg)])
		}
	}

	require.Equal(t, 0, int(c.GetVertexLocation(sa)), "precondition: A starts at its default shard 0")

	// No vertex in this graph has a reason to move, so the result does not depend on the visit order.
	result, _ := c.CLPAPartition()

	require.Equal(t, 0, int(c.GetVertexLocation(sa)), "the own shard scores highest, A must stay")
	require.NotContains(t, result, sa.Addr)
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
