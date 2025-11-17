package partition

import "github.com/HuangLab-SYSU/block-emulator/pkg/core/account"

// Vertex is the account in the blockchain.
type Vertex struct {
	Addr account.Address // account address
	// else
}

// Graph is to describe the accounts / transactions in the blockchain.
// vertex - account, edge - transaction
type Graph struct {
	VertexSet map[Vertex]struct{} // the set of vertexes
	EdgeSet   map[Vertex][]Vertex // to record the edges (transactions) between vertexes (accounts)
}

func NewGraph() *Graph {
	return &Graph{
		VertexSet: make(map[Vertex]struct{}),
		EdgeSet:   make(map[Vertex][]Vertex),
	}
}

// AddVertex adds the vertexes in the graph
func (g *Graph) AddVertex(v Vertex) {
	g.VertexSet[v] = struct{}{}
}

// AddEdge adds the edges in the graph
func (g *Graph) AddEdge(u, v Vertex) {
	// add vertexes first
	g.AddVertex(u)
	g.AddVertex(v)

	// non-direct graph, the weight of each edge is 1
	// There can exist repeated edges.
	g.EdgeSet[u] = append(g.EdgeSet[u], v)
	g.EdgeSet[v] = append(g.EdgeSet[v], u)
}
