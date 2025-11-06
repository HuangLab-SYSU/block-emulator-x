package partition

import (
	"log/slog"
	"math"
	"slices"
)

const updateThreshold = 50

// CLPAState is the state of constraint label propagation algorithm
type CLPAState struct {
	netGraph         *Graph         // 需运行CLPA算法的图
	partitionMap     map[Vertex]int // 记录分片信息的 map，某个节点属于哪个分片
	shardWeight      []float64      // total weight of edges associated with label k
	vertexNumInShard []int          // Shard 内节点的数目
	minShardWeight   float64        // 最少的 Shard 邻接边数，最小的 total weight of edges associated with label k

	crossShardEdgeNum float64 // 跨分片边的总数，与论文算法无关，用于评估性能

	maxIterations int     // 最大迭代次数，constraint，对应论文中的\tau
	weightPenalty float64 // 权重惩罚，对应论文中的 beta
	shardNum      int     // 分片数目
}

func NewCLPAState(wp float64, maxIterations, shardNum int) *CLPAState {
	return &CLPAState{
		netGraph:         NewGraph(),
		partitionMap:     make(map[Vertex]int),
		shardWeight:      make([]float64, shardNum),
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
		cs.vertexNumInShard[cs.partitionMap[v]]++
	}
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
}

// GetVertexLocation 获取节点所属的分片
func (cs *CLPAState) GetVertexLocation(v Vertex) int {
	return cs.partitionMap[v]
}

// computeEdges2Shard 根据当前划分，计算 Wk，即 shardWeight
func (cs *CLPAState) computeEdges2Shard() {
	cs.shardWeight = make([]float64, cs.shardNum)
	cs.crossShardEdgeNum = 0
	crossEdge := make([]int, cs.shardNum)
	interEdge := make([]int, cs.shardNum)
	cs.minShardWeight = math.MaxInt

	for v, lst := range cs.netGraph.EdgeSet {
		// 获取节点 v 所属的shard
		vShard := cs.partitionMap[v]
		for _, u := range lst {
			// 同上，获取节点 u 所属的shard
			uShard := cs.partitionMap[u]
			if vShard != uShard {
				// 判断节点 v, u 不属于同一分片，则对应的 shardWeight 加一
				// 根据论文公式 (5) 的前半部分，仅计算出度
				crossEdge[vShard]++
			} else {
				// vShard 内部边加一
				interEdge[vShard]++
			}
		}
	}

	for _, val := range crossEdge {
		// 因为一条边被插入两次，所以需要除以 2
		cs.crossShardEdgeNum += float64(val) / 2
	}

	for i := range cs.shardWeight {
		// 根据论文共识 (5)，计算 Lk(x)
		cs.shardWeight[i] = float64(crossEdge[i]) + float64(interEdge[i])/2
	}
	// 修改 minShardWeight
	cs.minShardWeight = slices.Min(cs.shardWeight)
}

// changeShardRecompute 在账户所属分片变动时，重新计算各个参数
// This is a faster function than before.
func (cs *CLPAState) changeShardRecompute(v Vertex, old int) {
	newShard := cs.partitionMap[v]
	for _, u := range cs.netGraph.EdgeSet[v] {
		uShard := cs.partitionMap[u]
		if uShard != newShard && uShard != old {
			cs.shardWeight[newShard] += 1.0
			cs.shardWeight[old] -= 1.0
		} else if uShard == newShard {
			cs.shardWeight[old] -= 1.0
		} else {
			cs.shardWeight[newShard] += 1.0
		}
	}

	cs.minShardWeight = slices.Min(cs.shardWeight)
}

// CLPAPartition 运行 CLPA 划分算法
func (cs *CLPAState) CLPAPartition() (map[Vertex]int, float64) {
	cs.computeEdges2Shard()
	slog.Info("Before running CLPA", "cross-shard edge number: ", cs.crossShardEdgeNum)

	ret := make(map[Vertex]int)
	updateTimes := make(map[Vertex]int)

	for range cs.maxIterations { // 第一层循环控制算法次数，constraint
		for v := range cs.netGraph.VertexSet {
			if updateTimes[v] >= updateThreshold {
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
				ret[v] = maxScoreShard
				updateTimes[v]++
				// 重新计算 vertexNumInShard
				cs.vertexNumInShard[vNowShard]--
				cs.vertexNumInShard[maxScoreShard]++
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

	return ret, cs.crossShardEdgeNum
}

// getShardScore 计算 将节点 v 放入 uShard 所产生的 score
func (cs *CLPAState) getShardScore(v Vertex, uShard int) float64 {
	// 节点 v 的出度
	vOutdegree := len(cs.netGraph.EdgeSet[v])
	// uShard 与节点 v 相连的边数
	edge2uShard := 0

	for _, item := range cs.netGraph.EdgeSet[v] {
		if cs.partitionMap[item] == uShard {
			edge2uShard++
		}
	}

	score := float64(edge2uShard) / float64(vOutdegree) * (1 - cs.weightPenalty*cs.shardWeight[uShard]/cs.minShardWeight)

	return score
}
