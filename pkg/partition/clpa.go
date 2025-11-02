package partition

import (
	"bytes"
	"encoding/gob"
	"errors"
	"log"
	"log/slog"
	"math"
	"strconv"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
)

// CLPAState is the state of constraint label propagation algorithm
type CLPAState struct {
	NetGraph          Graph          // 需运行CLPA算法的图
	PartitionMap      map[Vertex]int // 记录分片信息的 map，某个节点属于哪个分片
	Edges2Shard       []int          // Shard 相邻接的边数，对应论文中的 total weight of edges associated with label k
	VertexNumInShard  []int          // Shard 内节点的数目
	WeightPenalty     float64        // 权重惩罚，对应论文中的 beta
	MinEdges2Shard    int            // 最少的 Shard 邻接边数，最小的 total weight of edges associated with label k
	MaxIterations     int            // 最大迭代次数，constraint，对应论文中的\tau
	CrossShardEdgeNum int            // 跨分片边的总数
	ShardNum          int            // 分片数目
	GraphHash         []byte
}

func (cs *CLPAState) Encode() ([]byte, error) {
	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)
	if err := enc.Encode(cs); err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// AddVertex 加入节点，需要将它默认归到一个分片中
func (cs *CLPAState) AddVertex(v Vertex) {
	cs.NetGraph.AddVertex(v)

	if val, ok := cs.PartitionMap[v]; !ok {
		cs.PartitionMap[v] = int(DefaultAccountLoc(account.Address([]byte(v.Addr)), int64(cs.ShardNum)))
	} else {
		cs.PartitionMap[v] = val
	}

	cs.VertexNumInShard[cs.PartitionMap[v]] += 1 // 此处可以批处理完之后再修改 VertexNumInShard 参数
	// 当然也可以不处理，因为 CLPA 算法运行前会更新最新的参数
}

// AddEdge 加入边，需要将它的端点（如果不存在）默认归到一个分片中
func (cs *CLPAState) AddEdge(u, v Vertex) {
	// 如果没有点，则增加边，权恒定为 1
	if _, ok := cs.NetGraph.VertexSet[u]; !ok {
		cs.AddVertex(u)
	}

	if _, ok := cs.NetGraph.VertexSet[v]; !ok {
		cs.AddVertex(v)
	}

	cs.NetGraph.AddEdge(u, v)
	// 可以批处理完之后再修改 Edges2Shard 等参数
	// 当然也可以不处理，因为 CLPA 算法运行前会更新最新的参数
}

// CopyCLPA 深拷贝CLPA状态
func (cs *CLPAState) CopyCLPA(src CLPAState) {
	cs.NetGraph.CopyGraph(src.NetGraph)

	cs.PartitionMap = make(map[Vertex]int)
	for v := range src.PartitionMap {
		cs.PartitionMap[v] = src.PartitionMap[v]
	}

	cs.Edges2Shard = make([]int, src.ShardNum)
	copy(cs.Edges2Shard, src.Edges2Shard)
	cs.VertexNumInShard = src.VertexNumInShard
	cs.WeightPenalty = src.WeightPenalty
	cs.MinEdges2Shard = src.MinEdges2Shard
	cs.MaxIterations = src.MaxIterations
	cs.ShardNum = src.ShardNum
}

// ComputeEdges2Shard 根据当前划分，计算 Wk，即 Edges2Shard
func (cs *CLPAState) ComputeEdges2Shard() {
	cs.Edges2Shard = make([]int, cs.ShardNum)
	interEdge := make([]int, cs.ShardNum)
	cs.MinEdges2Shard = math.MaxInt

	for idx := 0; idx < cs.ShardNum; idx++ {
		cs.Edges2Shard[idx] = 0
		interEdge[idx] = 0
	}

	for v, lst := range cs.NetGraph.EdgeSet {
		// 获取节点 v 所属的shard
		vShard := cs.PartitionMap[v]
		for _, u := range lst {
			// 同上，获取节点 u 所属的shard
			uShard := cs.PartitionMap[u]
			if vShard != uShard {
				// 判断节点 v, u 不属于同一分片，则对应的 Edges2Shard 加一
				// 仅计算入度，这样不会重复计算
				cs.Edges2Shard[uShard] += 1
			} else {
				interEdge[uShard]++
			}
		}
	}

	cs.CrossShardEdgeNum = 0
	for _, val := range cs.Edges2Shard {
		cs.CrossShardEdgeNum += val
	}

	cs.CrossShardEdgeNum /= 2

	for idx := 0; idx < cs.ShardNum; idx++ {
		cs.Edges2Shard[idx] += interEdge[idx] / 2
	}
	// 修改 MinEdges2Shard, CrossShardEdgeNum
	for _, val := range cs.Edges2Shard {
		if cs.MinEdges2Shard > val {
			cs.MinEdges2Shard = val
		}
	}
}

// changeShardRecompute 在账户所属分片变动时，重新计算各个参数，faster
func (cs *CLPAState) changeShardRecompute(v Vertex, old int) {
	newShard := cs.PartitionMap[v]
	for _, u := range cs.NetGraph.EdgeSet[v] {
		neighborShard := cs.PartitionMap[u]
		if neighborShard != newShard && neighborShard != old {
			cs.Edges2Shard[newShard]++
			cs.Edges2Shard[old]--
		} else if neighborShard == newShard {
			cs.Edges2Shard[old]--
			cs.CrossShardEdgeNum--
		} else {
			cs.Edges2Shard[newShard]++
			cs.CrossShardEdgeNum++
		}
	}

	cs.MinEdges2Shard = math.MaxInt
	// 修改 MinEdges2Shard, CrossShardEdgeNum
	for _, val := range cs.Edges2Shard {
		if cs.MinEdges2Shard > val {
			cs.MinEdges2Shard = val
		}
	}
}

// InitCLPAState 设置参数
func (cs *CLPAState) InitCLPAState(wp float64, mIter, sn int) {
	cs.WeightPenalty = wp
	cs.MaxIterations = mIter
	cs.ShardNum = sn
	cs.VertexNumInShard = make([]int, cs.ShardNum)
	cs.PartitionMap = make(map[Vertex]int)
}

// InitPartition 初始化划分，使用节点地址的尾数划分，应该保证初始化的时候不会出现空分片
func (cs *CLPAState) InitPartition() {
	// 设置划分默认参数
	cs.VertexNumInShard = make([]int, cs.ShardNum)

	cs.PartitionMap = make(map[Vertex]int)
	for v := range cs.NetGraph.VertexSet {
		va := v.Addr[len(v.Addr)-8:]

		num, err := strconv.ParseInt(va, 16, 64)
		if err != nil {
			log.Panic()
		}

		cs.PartitionMap[v] = int(num) % cs.ShardNum
		cs.VertexNumInShard[cs.PartitionMap[v]] += 1
	}

	cs.ComputeEdges2Shard() // 删掉会更快一点，但是这样方便输出（毕竟只执行一次Init，也快不了多少）
}

// StableInitPartition 不会出现空分片的初始化划分
func (cs *CLPAState) StableInitPartition() error {
	// 设置划分默认参数
	if cs.ShardNum > len(cs.NetGraph.VertexSet) {
		return errors.New("too many shards, number of shards should be less than nodes. ")
	}

	cs.VertexNumInShard = make([]int, cs.ShardNum)
	cs.PartitionMap = make(map[Vertex]int)

	cnt := 0
	for v := range cs.NetGraph.VertexSet {
		cs.PartitionMap[v] = cnt % cs.ShardNum
		cs.VertexNumInShard[cs.PartitionMap[v]] += 1
		cnt++
	}

	cs.ComputeEdges2Shard() // 删掉会更快一点，但是这样方便输出（毕竟只执行一次Init，也快不了多少）

	return nil
}

// 计算 将节点 v 放入 uShard 所产生的 score
func (cs *CLPAState) getShardScore(v Vertex, uShard int) float64 {
	var score float64
	// 节点 v 的出度
	vOutdegree := len(cs.NetGraph.EdgeSet[v])
	// uShard 与节点 v 相连的边数
	Edges2UShard := 0

	for _, item := range cs.NetGraph.EdgeSet[v] {
		if cs.PartitionMap[item] == uShard {
			Edges2UShard += 1
		}
	}

	score = float64(Edges2UShard) / float64(vOutdegree) * (1 - cs.WeightPenalty*float64(cs.Edges2Shard[uShard])/float64(cs.MinEdges2Shard))

	return score
}

// CLPAPartition CLPA 划分算法
func (cs *CLPAState) CLPAPartition() (map[string]uint64, int) {
	cs.ComputeEdges2Shard()
	slog.Info("Before running CLPA", "cross-shard edge number: ", cs.CrossShardEdgeNum)

	res := make(map[string]uint64)
	updateThreshold := make(map[string]int)

	for iter := 0; iter < cs.MaxIterations; iter += 1 { // 第一层循环控制算法次数，constraint
		for v := range cs.NetGraph.VertexSet {
			if updateThreshold[v.Addr] >= 50 {
				continue
			}

			neighborShardScore := make(map[int]float64)
			maxScore := -9999.0

			vNowShard, maxScoreShard := cs.PartitionMap[v], cs.PartitionMap[v]
			for _, u := range cs.NetGraph.EdgeSet[v] {
				uShard := cs.PartitionMap[u]
				// 对于属于 uShard 的邻居，仅需计算一次
				if _, computed := neighborShardScore[uShard]; !computed {
					neighborShardScore[uShard] = cs.getShardScore(v, uShard)
					if maxScore < neighborShardScore[uShard] {
						maxScore = neighborShardScore[uShard]
						maxScoreShard = uShard
					}
				}
			}

			if vNowShard != maxScoreShard && cs.VertexNumInShard[vNowShard] > 1 {
				cs.PartitionMap[v] = maxScoreShard
				res[v.Addr] = uint64(maxScoreShard)
				updateThreshold[v.Addr]++
				// 重新计算 VertexNumInShard
				cs.VertexNumInShard[vNowShard] -= 1
				cs.VertexNumInShard[maxScoreShard] += 1
				// 重新计算Wk
				cs.changeShardRecompute(v, vNowShard)
			}
		}
	}

	for sid, n := range cs.VertexNumInShard {
		slog.Info("Vertex number in shard", "sharID", sid, "vertex number", n)
	}

	cs.ComputeEdges2Shard()
	slog.Info("After running CLPA", "cross-shard edge number", cs.CrossShardEdgeNum)

	return res, cs.CrossShardEdgeNum
}

func (cs *CLPAState) EraseEdges() {
	cs.NetGraph.EdgeSet = make(map[Vertex][]Vertex)
}
