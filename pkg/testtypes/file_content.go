package testtypes

import (
	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/pkg/ifaces"
)

// newFileContentTest reads a file on the node and applies the standard
// evaluate block to its content (match, contains, regex, json_path, ...).
// Options: filename (alias path). With no evaluate block, the test asserts
// the file is readable.
func newFileContentTest(base BaseTest, opts map[string]interface{}) (ifaces.Test, error) {
	path, err := requiredString(base.name, opts, "filename", "path")
	if err != nil {
		return nil, err
	}

	spec, err := evaluateSpec(base.name, opts)
	if err != nil {
		return nil, err
	}
	evaluations, err := eval.Parse(spec)
	if err != nil {
		return nil, err
	}
	if len(evaluations) == 0 {
		evaluations["readable"] = &eval.EvaluateExitCode{Expected: []int{0}}
	}

	base.evaluations = evaluations
	return &commandTest{
		BaseTest: base,
		command:  "cat " + helpers.ShellQuote(path),
	}, nil
}
