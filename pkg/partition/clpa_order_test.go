package partition

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCLPAWithOrder checks that the visit order really reaches the sweep: the same order twice gives
// the same table, a different order gives a different one.
func TestCLPAWithOrder(t *testing.T) {
	edges := fixedGraph()

	build := func() *CLPAState {
		c := NewCLPAState(w, maxIterations, shardNum)
		for _, e := range edges {
			c.AddEdge(e[0], e[1])
		}
		return c
	}

	seeded := func(seed int64) VisitOrder {
		r := rand.New(rand.NewSource(seed))
		return func(vertices []Vertex) []Vertex {
			r.Shuffle(len(vertices), func(i, j int) {
				vertices[i], vertices[j] = vertices[j], vertices[i]
			})
			return vertices
		}
	}

	first, _ := build().CLPAPartitionWithOrder(seeded(1))
	second, _ := build().CLPAPartitionWithOrder(seeded(1))
	require.Equal(t, first, second)

	third, _ := build().CLPAPartitionWithOrder(seeded(2))
	require.NotEqual(t, first, third)

	// The order is called once per sweep and always receives the full vertex set.
	c := build()
	calls := 0
	sizes := make([]int, 0, maxIterations)

	counting := func(vertices []Vertex) []Vertex {
		calls++
		sizes = append(sizes, len(vertices))
		return vertices
	}

	result, _ := c.CLPAPartitionWithOrder(counting)
	require.Equal(t, maxIterations, calls)

	for _, n := range sizes {
		require.Equal(t, len(c.netGraph.EdgeSet), n)
	}

	for _, loc := range result {
		require.True(t, loc >= 0 && loc < shardNum, "location %d out of range", loc)
	}
}

// fixedGraph is a reproducible random graph, standing in for generateRandomEdges: the tests above
// compare two runs over the same graph, so the graph itself must not differ between them.
func fixedGraph() [][]Vertex {
	src := rand.New(rand.NewSource(20260916))

	vertexes := make([]Vertex, vertexNum)
	for i := range vertexes {
		var addr [20]byte
		src.Read(addr[:])
		vertexes[i] = Vertex{Addr: addr}
	}

	ret := make([][]Vertex, 0, edgesNum)
	for range edgesNum {
		a, b := src.Intn(vertexNum), src.Intn(vertexNum)
		if a == b {
			continue // avoid a self-loop tx
		}
		ret = append(ret, []Vertex{vertexes[a], vertexes[b]})
	}

	return ret
}

const (
	testClusters = 3
	testPerClus  = 20
)

// testAddr puts the index in the last four bytes of the address, which is what the default
// placement takes modulo. The other bytes stay zero: nothing else reads the address.
func testAddr(i int) [20]byte {
	var a [20]byte
	binary.BigEndian.PutUint32(a[16:], uint32(i))
	return a
}

// buildTestState adds the same set of edges every time: a ring inside each cluster and one edge
// between consecutive clusters. The insertion order is fixed too.
func buildTestState(t *testing.T) *CLPAState {
	t.Helper()

	state := NewCLPAState(w, maxIterations, shardNum)

	for c := range testClusters {
		base := c * testPerClus
		for i := range testPerClus {
			u := Vertex{Addr: testAddr(base + i)}
			v := Vertex{Addr: testAddr(base + (i+1)%testPerClus)}
			state.AddEdge(u, v)
		}
	}
	for c := range testClusters {
		u := Vertex{Addr: testAddr(c * testPerClus)}
		v := Vertex{Addr: testAddr(((c + 1) % testClusters) * testPerClus)}
		state.AddEdge(u, v)
	}
	require.Len(t, state.netGraph.EdgeSet, testClusters*testPerClus)

	return state
}

// sameMap compares two partition tables; an address present on one side only counts as different.
func sameMap(a, b map[[20]byte]int) bool {
	if len(a) != len(b) {
		return false
	}
	for addr, shard := range a {
		if other, ok := b[addr]; !ok || other != shard {
			return false
		}
	}

	return true
}

func permuted(vertices []Vertex) []Vertex {
	out := make([]Vertex, len(vertices))
	copy(out, vertices)
	return out
}

// TestSeededOrderKeepsTheContract checks the VisitOrder contract: none added, none dropped, none
// repeated.
func TestSeededOrderKeepsTheContract(t *testing.T) {
	vertices := make([]Vertex, testClusters*testPerClus)
	for i := range vertices {
		vertices[i] = Vertex{Addr: testAddr(i)}
	}

	got := SeededOrder(7)(permuted(vertices))
	require.Len(t, got, len(vertices))

	seen := make(map[[20]byte]bool, len(got))
	for _, v := range got {
		require.False(t, seen[v.Addr], "vertex %x appears twice in the visit order", v.Addr[:4])
		seen[v.Addr] = true
	}
}

// TestSeededOrderIsReproducible: one seed and one graph give the same table twice.
func TestSeededOrderIsReproducible(t *testing.T) {
	first, _ := buildTestState(t).CLPAPartitionWithOrder(SeededOrder(42))
	second, _ := buildTestState(t).CLPAPartitionWithOrder(SeededOrder(42))

	require.True(t, sameMap(first, second), "one seed produced two different partition tables")
	require.NotEmpty(t, first, "the table is empty, so this assertion tests nothing")
}

// TestSeededOrderDependsOnSeed guards the other direction: an implementation that ignores the seed
// must not pass on the strength of the test above.
func TestSeededOrderDependsOnSeed(t *testing.T) {
	seed42, _ := buildTestState(t).CLPAPartitionWithOrder(SeededOrder(42))
	seed43, _ := buildTestState(t).CLPAPartitionWithOrder(SeededOrder(43))

	require.False(t, sameMap(seed42, seed43),
		"two different seeds produced the same partition table, the seed never reached the visit order")

	vertices := make([]Vertex, testClusters*testPerClus)
	for i := range vertices {
		vertices[i] = Vertex{Addr: testAddr(i)}
	}
	x := SeededOrder(42)(permuted(vertices))
	y := SeededOrder(42)(permuted(vertices))
	require.Equal(t, x, y)

	z := SeededOrder(43)(permuted(vertices))
	require.NotEqual(t, x, z, "two different seeds produced the same permutation")
}

// TestNilOrderStaysUpstream checks that the path without an order still works; the two runs are not
// compared, because an unseeded visit order is not reproducible by nature.
func TestNilOrderStaysUpstream(t *testing.T) {
	cases := map[string]func(*CLPAState) (map[[20]byte]int, float64){
		"CLPAPartition": func(s *CLPAState) (map[[20]byte]int, float64) { return s.CLPAPartition() },
		"CLPAPartitionWithOrder(nil)": func(s *CLPAState) (map[[20]byte]int, float64) {
			return s.CLPAPartitionWithOrder(nil)
		},
	}

	for name, run := range cases {
		got, _ := run(buildTestState(t))
		require.NotEmpty(t, got, "%s moved nobody, so this assertion tests nothing", name)
		for addr, shard := range got {
			require.True(t, shard >= 0 && shard < shardNum,
				"%s put %x into out-of-range shard %d", name, addr[:4], shard)
		}
	}
}

// TestCLPATieBreak checks the tie-break on equal scores: A has three out-of-shard neighbours with
// the same score, and only the edge insertion order changes, so the result must stay the same. On a
// chain the adjacency list order is decided by the arrival order of the shard blocks.
//
// Graph: A sits in shard 0 with no neighbour inside it, and one neighbour each in shard 1, 2 and 3;
// each of those shards holds a 6-vertex ring, and shard 0 holds two more vertices joined by an edge
// so that moving A out cannot empty the shard. Then W = (4, 4, 4, 4), and A scores
// 1/3 · (1 − 0.5·4/4) = 0.1667 against shard 1, 2 and 3 alike, so the tie must go to shard 1.
func TestCLPATieBreak(t *testing.T) {
	sa := Vertex{Addr: testAddr(100)} // 100 mod 4 = 0
	sb := Vertex{Addr: testAddr(4)}   // 4 mod 4 = 0
	sc := Vertex{Addr: testAddr(8)}   // 8 mod 4 = 0

	ring := func(res int) []Vertex {
		out := make([]Vertex, 6)
		for j := range out {
			out[j] = Vertex{Addr: testAddr(4*(j+1) + res)}
		}
		return out
	}
	sp, sq, sr := ring(1), ring(2), ring(3)

	// crossOrder is the insertion order of A's three cross-shard edges; the two graphs are identical
	// in every other respect.
	build := func(crossOrder []Vertex) *CLPAState {
		c := NewCLPAState(w, 1, shardNum)
		c.AddEdge(sb, sc)
		for _, other := range crossOrder {
			c.AddEdge(sa, other)
		}
		for _, rg := range [][]Vertex{sp, sq, sr} {
			for j := range rg {
				c.AddEdge(rg[j], rg[(j+1)%len(rg)])
			}
		}
		return c
	}

	// Visit A first and sweep once only: A's outcome is then decided by the initial state alone.
	aFirst := func(vertices []Vertex) []Vertex {
		out := make([]Vertex, 0, len(vertices))
		out = append(out, sa)
		for _, v := range vertices {
			if v != sa {
				out = append(out, v)
			}
		}
		return out
	}
	loc := func(c *CLPAState) int {
		c.CLPAPartitionWithOrder(aFirst)
		return int(c.GetVertexLocation(sa))
	}

	forward := []Vertex{sp[0], sq[0], sr[0]}
	reverse := []Vertex{sr[0], sq[0], sp[0]}

	require.Equal(t, 1, loc(build(forward)), "a tie must go to the lower shard id")
	require.Equal(t, 1, loc(build(reverse)), "the insertion order changed, the result must not")
}

// TestSeededOrderSweepsDiffer checks that seeding did not turn the 100 sweeps into 100 copies of the
// same sweep.
func TestSeededOrderSweepsDiffer(t *testing.T) {
	vertices := make([]Vertex, testClusters*testPerClus)
	for i := range vertices {
		vertices[i] = Vertex{Addr: testAddr(i)}
	}

	order := SeededOrder(20260916)
	slice := permuted(vertices)
	sweeps := make([][]byte, 3)
	for i := range sweeps {
		order(slice)
		sweeps[i] = make([]byte, 0, 20*len(slice))
		for _, v := range slice {
			sweeps[i] = append(sweeps[i], v.Addr[:]...)
		}
	}

	require.False(t, bytes.Equal(sweeps[0], sweeps[1]), "two adjacent sweeps got the same permutation")
	require.False(t, bytes.Equal(sweeps[1], sweeps[2]), "two adjacent sweeps got the same permutation")
}
