package internal

import (
	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/internal/helpers"
	"sort"
)

type Test interface {
	Name() string
	Run(updater formatters.TestCompleter) (results map[string]*eval.EvaluateResult, err error)
}

// CreateTests creates a slice of Test objects from a slice of TestConfig objects
func CreateTests(configs []*config.TestConfig, nodes map[string]Node) (tests []Test, err error) {
	tests = make([]Test, 0)

	// Sort tests by order
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].Order < configs[j].Order
	})

	// Parse the configurations into test objects
	for _, cfg := range configs {

		// Find the node
		node, ok := nodes[cfg.Node]
		if !ok {
			return nil, ErrNodeNotFound
		}

		// Process the type and pass the options to the test type constructor

		base := BaseTest{
			name:     cfg.Name,
			node:     node,
			testType: cfg.Type,
			setup:    cfg.Setup,
			teardown: cfg.Teardown,
		}

		var test Test
		switch cfg.Type {
		case TypeExecute:
			test, err = NewExecuteTest(base, &cfg.Options)
		case TypeExists:
			return nil, helpers.WrapError("Test type not implemented")
		case TypeFileContent:
			return nil, helpers.WrapError("Test type not implemented")
		case TypeFileHash:
			return nil, helpers.WrapError("Test type not implemented")
		case TypeHTTPRequest:
			return nil, helpers.WrapError("Test type not implemented")
		case TypePing:
			return nil, helpers.WrapError("Test type not implemented")
		case TypePortCheck:
			return nil, helpers.WrapError("Test type not implemented")
		case TypeResource:
			return nil, helpers.WrapError("Test type not implemented")
		case TypeServiceStatus:
			return nil, helpers.WrapError("Test type not implemented")
		default:
			return nil, ErrUnknownTestType
		}

		if err != nil {
			return nil, err
		}

		tests = append(tests, test)

	}
	return tests, nil
}
