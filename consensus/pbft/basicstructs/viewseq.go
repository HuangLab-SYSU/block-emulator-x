package basicstructs

type ViewSeq struct {
	// View and Seq of PBFT consensus
	View, Seq int64
}

func (v ViewSeq) Compare(other ViewSeq) int {
	if v.View != other.View {
		if v.View < other.View {
			return -1
		}

		return 1
	}

	if v.Seq != other.Seq {
		if v.Seq < other.Seq {
			return -1
		}

		return 1
	}

	return 0
}
