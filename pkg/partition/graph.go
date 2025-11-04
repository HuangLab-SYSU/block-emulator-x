package partition

// Vertex 图中的结点，即区块链网络中参与交易的账户
type Vertex struct {
	Addr [20]byte // 账户地址
	// 其他
}

// Graph 描述当前区块链交易集合的图
type Graph struct {
	VertexSet map[Vertex]struct{} // 节点集合，其实是 set
	EdgeSet   map[Vertex][]Vertex // 记录节点与节点间是否存在交易，邻接表
}

func NewGraph() *Graph {
	return &Graph{
		VertexSet: make(map[Vertex]struct{}),
		EdgeSet:   make(map[Vertex][]Vertex),
	}
}

// AddVertex 增加图中的点
func (g *Graph) AddVertex(v Vertex) {
	g.VertexSet[v] = struct{}{}
}

// AddEdge 增加图中的边
func (g *Graph) AddEdge(u, v Vertex) {
	// 如果没有点，则增加点
	g.AddVertex(u)
	g.AddVertex(v)

	// 无向图，使用双向边，权恒定为 1
	g.EdgeSet[u] = append(g.EdgeSet[u], v)
	g.EdgeSet[v] = append(g.EdgeSet[v], u)
}
