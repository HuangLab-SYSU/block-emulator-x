package partition

import (
	"log/slog"
	"math"
	"slices"
	"sync"
)

const updateThreshold = 50

// CLPAState is the state of constraint label propagation algorithm
type CLPAState struct {
	netGraph         *Graph         // the graph to run CLPA algorithm.
	partitionMap     map[Vertex]int // records the locations of all vertexes.
	shardWeight      []float64      // total weight of edges associated with label k.
	vertexNumInShard []int          // the number of vertexes in each shard.
	minShardWeight   float64        // the min values in shardWeight, i.e., min(total weight of edges associated with label k)

	crossShardEdgeNum float64 // the total number of all cross-shard edges, which is used for evaluating the performance of CLPA.

	maxIterations int     // hyperparameter, the max iteration times, for the \tau in the paper.
	weightPenalty float64 // hyperparameter, the weight penalty, for the beta in the paper.
	shardNum      int     // hyperparameter, the number of shards.

	mux sync.Mutex
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

// AddEdge adds edges.
// if the vertexes of this edge are not existed, add them first.
func (cs *CLPAState) AddEdge(u, v Vertex) {
	cs.mux.Lock()
	defer cs.mux.Unlock()

	if _, ok := cs.netGraph.VertexSet[u]; !ok {
		cs.addVertex(u)
	}

	if _, ok := cs.netGraph.VertexSet[v]; !ok {
		cs.addVertex(v)
	}

	cs.netGraph.AddEdge(u, v)
}

// EraseEdges drops the accumulated edge set so that the next CLPA round is driven by the blocks of
// a single epoch only.
//
// EdgeSet alone is cleared. VertexSet and vertexNumInShard have to be kept: shardWeight is
// recomputed from EdgeSet on every round, and an account whose adjacency list is empty does not
// move. Clearing VertexSet would stop vertexNumInShard from being updated, the guard against
// emptying a shard in CLPAPartition would never fire, and the partition would silently stop
// working while the logs and result files still looked normal.
func (cs *CLPAState) EraseEdges() {
	cs.mux.Lock()
	defer cs.mux.Unlock()

	cs.netGraph.EdgeSet = make(map[Vertex][]Vertex)
}

// GetVertexLocation gets the location of a vertex
func (cs *CLPAState) GetVertexLocation(v Vertex) int {
	cs.mux.Lock()
	defer cs.mux.Unlock()

	return cs.getVertexLocation(v)
}

// computeEdges2Shard calculates shardWeight according to current graph.
func (cs *CLPAState) computeEdges2Shard() {
	cs.shardWeight = make([]float64, cs.shardNum)
	cs.crossShardEdgeNum = 0
	crossEdge := make([]int, cs.shardNum)
	interEdge := make([]int, cs.shardNum)
	cs.minShardWeight = math.MaxInt

	for v, lst := range cs.netGraph.EdgeSet {
		// get the shard of vertex v
		vShard := cs.getVertexLocation(v)

		for _, u := range lst {
			// get the shard of vertex u
			uShard := cs.getVertexLocation(u)
			if vShard != uShard {
				// if u and v are not in the same shard, increase shardWeight
				// according to the equation (5) in the paper, calculate out-degree only.
				crossEdge[vShard]++
			} else {
				// increase the inner-shard edge
				interEdge[vShard]++
			}
		}
	}

	for _, val := range crossEdge {
		// one edge will be inserted twice, thus val should be divided by 2.
		cs.crossShardEdgeNum += float64(val) / 2
	}

	for i := range cs.shardWeight {
		// according to the equation (5) in the paper, calculate Lk(x)
		cs.shardWeight[i] = float64(crossEdge[i]) + float64(interEdge[i])/2
	}
	// get the minShardWeight
	cs.minShardWeight = slices.Min(cs.shardWeight)

	// getShardScore divides by minShardWeight. A zero divisor does not panic, it poisons every
	// candidate: a shard with weight 0 scores 0/0 = NaN, the others score x/0 = +Inf and
	// 1-weightPenalty*Inf = -Inf. CLPAPartition compares candidates with a strict greater-than,
	// which is false against both NaN and -Inf, so no account moves and the round broadcasts an
	// empty table, outwardly indistinguishable from "CLPA converged". An epoch-scoped graph makes
	// this reachable, since a single epoch may hold no edge at all; it has to be loud, not silent.
	if cs.minShardWeight <= 0 {
		slog.Error("minShardWeight <= 0: this CLPA round would move nobody",
			"shardWeight", cs.shardWeight)
		cs.minShardWeight = 1
	}
}

// changeShardRecompute calculates each parameter when the locations of accounts are changed.
func (cs *CLPAState) changeShardRecompute(v Vertex, old int) {
	newShard := cs.getVertexLocation(v)

	for _, u := range cs.netGraph.EdgeSet[v] {
		uShard := cs.getVertexLocation(u)
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

// CLPAPartition runs CLPA.
func (cs *CLPAState) CLPAPartition() (map[[20]byte]int, float64) {
	cs.computeEdges2Shard()
	slog.Info("before running CLPA", "cross-shard edge number: ", cs.crossShardEdgeNum)

	// Both loops below iterate EdgeSet, not VertexSet. Only a vertex with edges can move: with
	// EdgeSet[v] empty the neighbour loop does not run, maxScoreShard stays vNowShard, and nothing
	// is written. EdgeSet is cleared once per epoch while VertexSet only grows, so iterating the
	// latter makes every repartition cost more as the run goes on. Both are map ranges, whose
	// order is randomised either way, so the result is unchanged.
	originalLoc := make(map[[20]byte]int, len(cs.netGraph.EdgeSet))
	ret := make(map[[20]byte]int)
	updateTimes := make(map[Vertex]int)

	for v := range cs.netGraph.EdgeSet {
		originalLoc[v.Addr] = cs.getVertexLocation(v)
	}

	for range cs.maxIterations { // first loop, constraint the max iterations
		for v := range cs.netGraph.EdgeSet {
			if updateTimes[v] >= updateThreshold {
				continue
			}

			neighborShardScore := make(map[int]float64)
			maxScore := -9999.0

			vNowShard, maxScoreShard := cs.getVertexLocation(v), cs.getVertexLocation(v)

			for _, u := range cs.netGraph.EdgeSet[v] {
				uShard := cs.getVertexLocation(u)
				if uShard == vNowShard {
					continue
				}
				// calculate only once for each neighbor shard.
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
				ret[v.Addr] = maxScoreShard
				updateTimes[v]++
				// re-calculate vertexNumInShard
				cs.vertexNumInShard[vNowShard]--
				cs.vertexNumInShard[maxScoreShard]++
				// re-calculate Wk
				cs.changeShardRecompute(v, vNowShard)
			}
		}
	}

	// Accounts may be migrated back into the original shard, thus prune the useless result in 'ret'.
	// originalLoc holds every address 'ret' can contain: 'ret' is written only when a vertex moves,
	// and a vertex without edges never moves, so every address in 'ret' is a key of EdgeSet.
	for acc, loc := range originalLoc {
		if ret[acc] == loc { // If the shard-location of this account is not changed
			delete(ret, acc)
		}
	}

	for sid, n := range cs.vertexNumInShard {
		slog.Info("vertex number in shard", "sharID", sid, "vertex number", n)
	}

	cs.computeEdges2Shard()
	slog.Info("after running CLPA", "cross-shard edge number", cs.crossShardEdgeNum)

	return ret, cs.crossShardEdgeNum
}

// addVertex adds vertexes to the graph and locate it to a default shard.
func (cs *CLPAState) addVertex(v Vertex) {
	cs.netGraph.AddVertex(v)

	// if this vertex is not added before, add it to this map
	if _, ok := cs.partitionMap[v]; !ok {
		loc := int(DefaultAccountLoc(v.Addr, int64(cs.shardNum)))
		cs.partitionMap[v] = loc
		cs.vertexNumInShard[loc]++
	}
}

// getShardScore calculate the earning score that moving vertex v to uShard.
func (cs *CLPAState) getShardScore(v Vertex, uShard int) float64 {
	// the outdegree of v
	vOutdegree := len(cs.netGraph.EdgeSet[v])
	// the number of edges in uShard that connects to vertex v
	edge2uShard := 0

	for _, item := range cs.netGraph.EdgeSet[v] {
		if cs.getVertexLocation(item) == uShard {
			edge2uShard++
		}
	}

	score := float64(
		edge2uShard,
	) / float64(
		vOutdegree,
	) * (1 - cs.weightPenalty*cs.shardWeight[uShard]/cs.minShardWeight)

	return score
}

func (cs *CLPAState) getVertexLocation(v Vertex) int {
	if val, ok := cs.partitionMap[v]; ok {
		return val
	}

	return int(DefaultAccountLoc(v.Addr, int64(cs.shardNum)))
}
