package message

const CLPARepartitionStartMessageType = "CLPARepartitionStart"

type CLPARepartitionStartMsg struct {
	Epoch       int64
	ModifiedMap map[[20]byte]int
}
