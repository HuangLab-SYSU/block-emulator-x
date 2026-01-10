package config

type NetworkCfg struct {
	Bandwidth         int64  `json:"bandwidth" yaml:"bandwidth"`
	Latency           int64  `json:"latency" yaml:"latency"`
	CommunicationMode string `json:"communication_mode" yaml:"communication_mode"`
}
