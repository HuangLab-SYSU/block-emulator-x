package vm

type Executor struct {
}

func NewExecutor() *Executor {
	return &Executor{}
}

func (e *Executor) Deploy() error {
	panic("implement me")
}

func (e *Executor) Call() ([]byte, error) {
	panic("implement me")
}
