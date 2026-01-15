package config

const (
	DirectConnMode = "direct"
	LibP2PConnMode = "libp2p"
)

type NetworkCfg struct {
	Bandwidth         int64  `json:"bandwidth"          yaml:"bandwidth"`
	Latency           int64  `json:"latency"            yaml:"latency"`
	CommunicationMode string `json:"communication_mode" yaml:"communication_mode"`
	LibP2PConnCfg     `       json:"libp2p"             yaml:"libp2p"`
}

type LibP2PConnCfg struct {
	BootstrapKeyFp string `json:"bootstrap_key_fp" yaml:"bootstrap_key_fp"`
	BootstrapPeer  string `json:"bootstrap_peer"   yaml:"bootstrap_peer"`
	BootstrapIP    string `json:"bootstrap_ip"     yaml:"bootstrap_ip"`
	BootstrapPort  int    `json:"bootstrap_port"   yaml:"bootstrap_port"`
}
