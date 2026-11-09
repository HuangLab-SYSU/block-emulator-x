package partition

import "math/rand"

// VisitOrder decides the order in which one sweep of CLPA visits the vertices.
//
// The argument is the vertex set reachable by that sweep, i.e. the keys of EdgeSet, given in
// ascending address order. The order must return exactly those vertices: none added, none dropped,
// none repeated. The same slice is reused across the sweeps of one CLPAPartitionWithOrder call, so
// an implementation may shuffle it in place; that is what lets one seed make a whole round
// reproducible.
//
// A nil VisitOrder keeps the upstream behaviour: Go's map iteration order, drawn afresh per sweep.
type VisitOrder func(vertices []Vertex) []Vertex

// SeededOrder returns a VisitOrder that shuffles by seed. The RNG is created once and consumed by
// the sweeps of one CLPAPartitionWithOrder call in turn: every sweep gets a different permutation,
// and the whole round is determined by the seed and the graph alone.
func SeededOrder(seed int64) VisitOrder {
	rng := rand.New(rand.NewSource(seed))

	return func(vertices []Vertex) []Vertex {
		rng.Shuffle(len(vertices), func(i, j int) {
			vertices[i], vertices[j] = vertices[j], vertices[i]
		})

		return vertices
	}
}
