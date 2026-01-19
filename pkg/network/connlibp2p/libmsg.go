package connlibp2p

type NodeRegisterMsg struct {
	ShardID int64
	NodeID  int64
	PeerID  string
}

type NodePeerBroadcastMsg struct {
	NodePeerMap map[int64]map[int64]string
}
