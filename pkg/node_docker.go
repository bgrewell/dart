package pkg

import "github.com/bgrewell/dart/internal/execution"

func NewDockerNode() Node {
	return &DockerNode{}
}

type DockerNode struct{}

func (d DockerNode) Execute(command string, options ...execution.ExecutionOption) (result *execution.ExecutionResult, err error) {
	//TODO implement me
	panic("implement me")
}

func (d DockerNode) Close() error {
	//TODO implement me
	panic("implement me")
}
