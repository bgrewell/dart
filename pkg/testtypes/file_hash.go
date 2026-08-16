package testtypes

import (
	"fmt"
	"strings"

	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/pkg/ifaces"
)

// hashAlgos maps evaluate keys to the checksum tool and digest length.
// Ordered so the generated command is deterministic.
var hashAlgos = []struct {
	name   string
	tool   string
	hexLen int
}{
	{"md5", "md5sum", 32},
	{"sha1", "sha1sum", 40},
	{"sha256", "sha256sum", 64},
}

// newFileHashTest verifies file checksums on the node. Options: filename
// (alias path); evaluate: md5 / sha1 / sha256 with expected hex digests
// (at least one required). Only the requested checksum tools run.
func newFileHashTest(base BaseTest, opts map[string]interface{}) (ifaces.Test, error) {
	path, err := requiredString(base.name, opts, "filename", "path")
	if err != nil {
		return nil, err
	}

	spec, err := evaluateSpec(base.name, opts)
	if err != nil {
		return nil, err
	}

	evaluations := make(map[string]eval.Evaluate)
	var commands []string
	for _, algo := range hashAlgos {
		raw, ok := spec[algo.name]
		if !ok {
			continue
		}
		expected, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be a hex string in test %q (got %T)", algo.name, base.name, raw)
		}
		expected = strings.ToLower(strings.TrimSpace(expected))
		if len(expected) != algo.hexLen || !hexRe.MatchString(expected) {
			return nil, fmt.Errorf("%s in test %q must be a %d-character hex digest", algo.name, base.name, algo.hexLen)
		}
		evaluations[algo.name] = &hashCheck{algo: algo.name, hexLen: algo.hexLen, expected: expected}
		commands = append(commands, fmt.Sprintf("%s -- %s", algo.tool, helpers.ShellQuote(path)))
	}

	for name := range spec {
		known := false
		for _, algo := range hashAlgos {
			if name == algo.name {
				known = true
				break
			}
		}
		if !known {
			// A matching digest already proves the contents byte for byte,
			// so a further check on the same file can only be redundant or
			// contradictory. It would also not mean what it appears to: the
			// result's stdout is the digest line, so `contains: hello` would
			// assert against "<digest>  <path>" rather than the file.
			// Assertions about contents belong on file_content.
			return nil, fmt.Errorf("check %q is not available in a file_hash test %q: a matching digest already proves the contents, "+
				"so this type accepts only md5, sha1, and sha256 — use file_content to assert on contents",
				name, base.name)
		}
	}

	if len(evaluations) == 0 {
		return nil, fmt.Errorf("at least one of md5/sha1/sha256 is required in test %q", base.name)
	}

	base.evaluations = evaluations
	return &commandTest{
		BaseTest: base,
		command:  strings.Join(commands, " && "),
	}, nil
}
