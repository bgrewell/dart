package internal

import (
	"github.com/bgrewell/dart/internal/eval"
)

type BaseTest struct {
	name        string
	node        Node
	testType    string
	setup       []string
	teardown    []string
	evaluations *map[string]eval.Evaluate
}
