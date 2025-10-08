package nodetopo

type NodeInfo struct {
	NodeID, ShardID int64
}

// NodeMapper tells about the Shard-Node relation and Shard-Account relation.
type NodeMapper interface {
	GetNodesInShard(shardID int64) ([]NodeInfo, error)
	GetLeader(shardID int64) (NodeInfo, error)
	ChangeLeader(shardID int64, info NodeInfo) error
	GetAllLeaders() ([]NodeInfo, error)
}
