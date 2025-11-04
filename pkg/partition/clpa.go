package partition

import (
	"log/slog"
	"math"
	"slices"
)

// CLPAState is the state of constraint label propagation algorithm
type CLPAState struct {
	netGraph          *Graph         // 需运行CLPA算法的图
	partitionMap      map[Vertex]int // 记录分片信息的 map，某个节点属于哪个分片
	edges2Shard       []int          // Shard 相邻接的边数，对应论文中的 total weight of edges associated with label k
	vertexNumInShard  []int          // Shard 内节点的数目
	minEdges2Shard    int            // 最少的 Shard 邻接边数，最小的 total weight of edges associated with label k
	crossShardEdgeNum int            // 跨分片边的总数

	maxIterations int     // 最大迭代次数，constraint，对应论文中的\tau
	weightPenalty float64 // 权重惩罚，对应论文中的 beta
	shardNum      int     // 分片数目
}

func NewCLPAState(wp float64, maxIterations, shardNum int) *CLPAState {
	return &CLPAState{
		netGraph:         NewGraph(),
		partitionMap:     make(map[Vertex]int),
		edges2Shard:      make([]int, shardNum),
		vertexNumInShard: make([]int, shardNum),

		weightPenalty: wp,
		maxIterations: maxIterations,
		shardNum:      shardNum,
	}
}

// AddVertex 加入节点，需要将它默认归到一个分片中
func (cs *CLPAState) AddVertex(v Vertex) {
	cs.netGraph.AddVertex(v)

	// if this vertex is not added before, add it to this map
	if _, ok := cs.partitionMap[v]; !ok {
		cs.partitionMap[v] = int(DefaultAccountLoc(v.Addr, int64(cs.shardNum)))
	}

	cs.vertexNumInShard[cs.partitionMap[v]] += 1 // 此处可以批处理完之后再修改 vertexNumInShard 参数
	// 当然也可以不处理，因为 CLPA 算法运行前会更新最新的参数
}

// AddEdge 加入边，需要将它的端点（如果不存在）默认归到一个分片中
func (cs *CLPAState) AddEdge(u, v Vertex) {
	// 如果没有点，则增加边，权恒定为 1
	if _, ok := cs.netGraph.VertexSet[u]; !ok {
		cs.AddVertex(u)
	}

	if _, ok := cs.netGraph.VertexSet[v]; !ok {
		cs.AddVertex(v)
	}

	cs.netGraph.AddEdge(u, v)
	// 可以批处理完之后再修改 edges2Shard 等参数
	// 当然也可以不处理，因为 CLPA 算法运行前会更新最新的参数
}

// computeEdges2Shard 根据当前划分，计算 Wk，即 edges2Shard
func (cs *CLPAState) computeEdges2Shard() {
	cs.edges2Shard = make([]int, cs.shardNum)
	interEdge := make([]int, cs.shardNum)
	cs.minEdges2Shard = math.MaxInt

	for idx := 0; idx < cs.shardNum; idx++ {
		cs.edges2Shard[idx] = 0
		interEdge[idx] = 0
	}

	for v, lst := range cs.netGraph.EdgeSet {
		// 获取节点 v 所属的shard
		vShard := cs.partitionMap[v]
		for _, u := range lst {
			// 同上，获取节点 u 所属的shard
			uShard := cs.partitionMap[u]
			if vShard != uShard {
				// 判断节点 v, u 不属于同一分片，则对应的 edges2Shard 加一
				// 仅计算入度，这样不会重复计算
				cs.edges2Shard[uShard] += 1
			} else {
				interEdge[uShard]++
			}
		}
	}

	cs.crossShardEdgeNum = 0
	for _, val := range cs.edges2Shard {
		cs.crossShardEdgeNum += val
	}

	cs.crossShardEdgeNum /= 2

	for idx := 0; idx < cs.shardNum; idx++ {
		cs.edges2Shard[idx] += interEdge[idx] / 2
	}
	// 修改 minEdges2Shard, crossShardEdgeNum
	for _, val := range cs.edges2Shard {
		if cs.minEdges2Shard > val {
			cs.minEdges2Shard = val
		}
	}
}

// changeShardRecompute 在账户所属分片变动时，重新计算各个参数
// This is a faster function than before.
func (cs *CLPAState) changeShardRecompute(v Vertex, old int) {
	newShard := cs.partitionMap[v]
	for _, u := range cs.netGraph.EdgeSet[v] {
		neighborShard := cs.partitionMap[u]
		if neighborShard != newShard && neighborShard != old {
			cs.edges2Shard[newShard]++
			cs.edges2Shard[old]--
		} else if neighborShard == newShard {
			cs.edges2Shard[old]--
			cs.crossShardEdgeNum--
		} else {
			cs.edges2Shard[newShard]++
			cs.crossShardEdgeNum++
		}
	}

	cs.minEdges2Shard = slices.Min(cs.edges2Shard)
}

// CLPAPartition 运行 CLPA 划分算法
func (cs *CLPAState) CLPAPartition() (map[Vertex]int, int) {
	cs.computeEdges2Shard()
	slog.Info("Before running CLPA", "cross-shard edge number: ", cs.crossShardEdgeNum)

	res := make(map[Vertex]int)
	updateThreshold := make(map[Vertex]int)

	for iter := 0; iter < cs.maxIterations; iter += 1 { // 第一层循环控制算法次数，constraint
		for v := range cs.netGraph.VertexSet {
			if updateThreshold[v] >= 50 {
				continue
			}

			neighborShardScore := make(map[int]float64)
			maxScore := -9999.0

			vNowShard, maxScoreShard := cs.partitionMap[v], cs.partitionMap[v]
			for _, u := range cs.netGraph.EdgeSet[v] {
				uShard := cs.partitionMap[u]
				// 对于属于 uShard 的邻居，仅需计算一次
				if _, computed := neighborShardScore[uShard]; !computed {
					neighborShardScore[uShard] = cs.getShardScore(v, uShard)
					if maxScore < neighborShardScore[uShard] {
						maxScore = neighborShardScore[uShard]
						maxScoreShard = uShard
					}
				}
			}

			if vNowShard != maxScoreShard && cs.vertexNumInShard[vNowShard] > 1 {
				cs.partitionMap[v] = maxScoreShard
				res[v] = maxScoreShard
				updateThreshold[v]++
				// 重新计算 vertexNumInShard
				cs.vertexNumInShard[vNowShard] -= 1
				cs.vertexNumInShard[maxScoreShard] += 1
				// 重新计算Wk
				cs.changeShardRecompute(v, vNowShard)
			}
		}
	}

	for sid, n := range cs.vertexNumInShard {
		slog.Info("Vertex number in shard", "sharID", sid, "vertex number", n)
	}

	cs.computeEdges2Shard()
	slog.Info("After running CLPA", "cross-shard edge number", cs.crossShardEdgeNum)

	return res, cs.crossShardEdgeNum
}

// getShardScore 计算 将节点 v 放入 uShard 所产生的 score
func (cs *CLPAState) getShardScore(v Vertex, uShard int) float64 {
	var score float64
	// 节点 v 的出度
	vOutdegree := len(cs.netGraph.EdgeSet[v])
	// uShard 与节点 v 相连的边数
	Edges2UShard := 0

	for _, item := range cs.netGraph.EdgeSet[v] {
		if cs.partitionMap[item] == uShard {
			Edges2UShard += 1
		}
	}

	score = float64(Edges2UShard) / float64(vOutdegree) * (1 - cs.weightPenalty*float64(cs.edges2Shard[uShard])/float64(cs.minEdges2Shard))

	return score
}
