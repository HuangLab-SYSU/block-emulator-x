package partition

import "fmt"

// Vertex 图中的结点，即区块链网络中参与交易的账户
type Vertex struct {
	Addr string // 账户地址
}

// Graph 描述当前区块链交易集合的图
type Graph struct {
	VertexSet map[Vertex]bool     // 节点集合，其实是 set
	EdgeSet   map[Vertex][]Vertex // 记录节点与节点间是否存在交易，邻接表
}

// ConstructVertex 创建节点
func (v *Vertex) ConstructVertex(s string) {
	v.Addr = s
}

// AddVertex 增加图中的点
func (g *Graph) AddVertex(v Vertex) {
	if g.VertexSet == nil {
		g.VertexSet = make(map[Vertex]bool)
	}

	g.VertexSet[v] = true
}

// AddEdge 增加图中的边
func (g *Graph) AddEdge(u, v Vertex) {
	// 如果没有点，则增加边，权恒定为 1
	if _, ok := g.VertexSet[u]; !ok {
		g.AddVertex(u)
	}

	if _, ok := g.VertexSet[v]; !ok {
		g.AddVertex(v)
	}

	if g.EdgeSet == nil {
		g.EdgeSet = make(map[Vertex][]Vertex)
	}
	// 无向图，使用双向边
	g.EdgeSet[u] = append(g.EdgeSet[u], v)
	g.EdgeSet[v] = append(g.EdgeSet[v], u)
}

// CopyGraph 复制图
func (g *Graph) CopyGraph(src Graph) {
	g.VertexSet = make(map[Vertex]bool)
	for v := range src.VertexSet {
		g.VertexSet[v] = true
	}

	if src.EdgeSet != nil {
		g.EdgeSet = make(map[Vertex][]Vertex)
		for v := range src.VertexSet {
			g.EdgeSet[v] = make([]Vertex, len(src.EdgeSet[v]))
			copy(g.EdgeSet[v], src.EdgeSet[v])
		}
	}
}

// PrintGraph 输出图
func (g *Graph) PrintGraph() {
	for v := range g.VertexSet {
		fmt.Print(v.Addr, " ")
		fmt.Print("edge:")

		for _, u := range g.EdgeSet[v] {
			fmt.Print(u.Addr)
		}

		fmt.Println()
	}

	fmt.Println()
}
