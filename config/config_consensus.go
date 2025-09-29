package config

type ConsensusCfg struct {
	ShardNum int64
	NodeNum  int64

	HandlerBufferSize int64
	BlockInterval     int64 // ms
}
