package steptypes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeStepOn builds a step bound to a specific node (makeStep uses a mock
// node; file steps need a local node to touch the filesystem).
func makeStepOn(t *testing.T, node ifaces.Node, stepType string, options map[string]interface{}) (ifaces.Step, error) {
	t.Helper()
	nodes := map[string]ifaces.Node{"test-node": node}
	configs := []*config.StepConfig{{
		Name: "transfer test",
		Node: config.NodeReference{"test-node"},
		Step: config.StepDetails{Type: stepType, Options: options},
	}}
	steps, err := CreateSteps(configs, nodes)
	if err != nil {
		return nil, err
	}
	require.Len(t, steps, 1)
	return steps[0], nil
}

func localNode(t *testing.T) ifaces.Node {
	t.Helper()
	opts := map[string]interface{}{"shell": "/bin/sh"}
	return nodetypes.NewLocalNode("transfer", ifaces.NodeOptions(&opts))
}

func TestFilePushCopiesContentAndMode(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "deploy.sh")
	require.NoError(t, os.WriteFile(source, []byte("#!/bin/sh\necho deployed\n"), 0755))
	dest := filepath.Join(dir, "out", "deploy.sh")

	step, err := makeStepOn(t, localNode(t), TypeFilePush, map[string]interface{}{
		"source": source, "dest": dest, "create_dir": true,
	})
	require.NoError(t, err)
	require.NoError(t, step.Run(formatters.NewMockTaskCompleter()))

	contents, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho deployed\n", string(contents))

	info, err := os.Stat(dest)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm(), "source permissions carry over so scripts stay executable")
}

func TestFilePushExplicitMode(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "secret.conf")
	require.NoError(t, os.WriteFile(source, []byte("token=abc\n"), 0644))
	dest := filepath.Join(dir, "secret.out")

	step, err := makeStepOn(t, localNode(t), TypeFilePush, map[string]interface{}{
		"source": source, "dest": dest, "mode": "0600",
	})
	require.NoError(t, err)
	require.NoError(t, step.Run(formatters.NewMockTaskCompleter()))

	info, err := os.Stat(dest)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestFilePushMissingSource(t *testing.T) {
	dir := t.TempDir()
	step, err := makeStepOn(t, localNode(t), TypeFilePush, map[string]interface{}{
		"source": filepath.Join(dir, "nope"), "dest": filepath.Join(dir, "out"),
	})
	require.NoError(t, err)
	err = step.Run(formatters.NewMockTaskCompleter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read source")
}

func TestFileFetchCopiesBack(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "app.log")
	require.NoError(t, os.WriteFile(source, []byte("line one\nline two\n"), 0644))
	dest := filepath.Join(dir, "artifacts", "app.log")

	step, err := makeStepOn(t, localNode(t), TypeFileFetch, map[string]interface{}{
		"source": source, "dest": dest, "create_dir": true,
	})
	require.NoError(t, err)
	require.NoError(t, step.Run(formatters.NewMockTaskCompleter()))

	contents, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "line one\nline two\n", string(contents))
}

func TestFileTemplateRenders(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "app.conf.tmpl")
	require.NoError(t, os.WriteFile(source, []byte("host = {{ .host }}\nport = {{ .port }}\n"), 0644))
	dest := filepath.Join(dir, "app.conf")

	step, err := makeStepOn(t, localNode(t), TypeFileTemplate, map[string]interface{}{
		"source": source, "dest": dest,
		"values": map[string]interface{}{"host": "10.0.0.5", "port": 8080},
	})
	require.NoError(t, err)
	require.NoError(t, step.Run(formatters.NewMockTaskCompleter()))

	contents, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "host = 10.0.0.5\nport = 8080\n", string(contents))
}

// A missing value is an error, not a silently empty config line.
func TestFileTemplateMissingValueFails(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "app.conf.tmpl")
	require.NoError(t, os.WriteFile(source, []byte("host = {{ .host }}\n"), 0644))

	step, err := makeStepOn(t, localNode(t), TypeFileTemplate, map[string]interface{}{
		"source": source, "dest": filepath.Join(dir, "out.conf"),
		"values": map[string]interface{}{},
	})
	require.NoError(t, err)
	err = step.Run(formatters.NewMockTaskCompleter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to render")
}

// A broken template fails at config time, before anything runs.
func TestFileTemplateInvalidAtConfigTime(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "bad.tmpl")
	require.NoError(t, os.WriteFile(source, []byte("{{ .unclosed \n"), 0644))

	_, err := makeStepOn(t, localNode(t), TypeFileTemplate, map[string]interface{}{
		"source": source, "dest": filepath.Join(dir, "out"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is invalid")
}

func TestFileTemplateMissingSourceAtConfigTime(t *testing.T) {
	dir := t.TempDir()
	_, err := makeStepOn(t, localNode(t), TypeFileTemplate, map[string]interface{}{
		"source": filepath.Join(dir, "nope.tmpl"), "dest": filepath.Join(dir, "out"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read template")
}

func TestTransferValidation(t *testing.T) {
	node := localNode(t)
	_, err := makeStepOn(t, node, TypeFilePush, map[string]interface{}{"dest": "/x"})
	assert.ErrorContains(t, err, "source is required")

	_, err = makeStepOn(t, node, TypeFilePush, map[string]interface{}{"source": "/x"})
	assert.ErrorContains(t, err, "dest is required")

	_, err = makeStepOn(t, node, TypeFileFetch, map[string]interface{}{"source": "/x"})
	assert.ErrorContains(t, err, "dest is required")
}

// A fetched artifact must not silently clobber a previous run's file.
func TestFileFetchRefusesToClobber(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "app.log")
	require.NoError(t, os.WriteFile(source, []byte("new\n"), 0644))
	dest := filepath.Join(dir, "existing.log")
	require.NoError(t, os.WriteFile(dest, []byte("previous run\n"), 0644))

	step, err := makeStepOn(t, localNode(t), TypeFileFetch, map[string]interface{}{
		"source": source, "dest": dest,
	})
	require.NoError(t, err)
	err = step.Run(formatters.NewMockTaskCompleter())
	require.Error(t, err)

	preserved, readErr := os.ReadFile(dest)
	require.NoError(t, readErr)
	assert.Equal(t, "previous run\n", string(preserved), "existing artifact preserved")

	// With overwrite it replaces the file
	overwriteStep, err := makeStepOn(t, localNode(t), TypeFileFetch, map[string]interface{}{
		"source": source, "dest": dest, "overwrite": true,
	})
	require.NoError(t, err)
	require.NoError(t, overwriteStep.Run(formatters.NewMockTaskCompleter()))
	replaced, _ := os.ReadFile(dest)
	assert.Equal(t, "new\n", string(replaced))
}

// A null value renders "<no value>" into a live config with no template
// error, so it is rejected at config time.
func TestFileTemplateNullValueRejected(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "app.conf.tmpl")
	require.NoError(t, os.WriteFile(source, []byte("port = {{ .port }}\n"), 0644))

	_, err := makeStepOn(t, localNode(t), TypeFileTemplate, map[string]interface{}{
		"source": source, "dest": filepath.Join(dir, "out.conf"),
		"values": map[string]interface{}{"port": nil},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is null")
}

// Content far beyond the shell's per-argument limit round-trips through
// the chunked remote writer.
func TestRemoteWriteLargeContent(t *testing.T) {
	shellOpts := map[string]interface{}{"shell": "/bin/sh"}
	node := nodetypes.NewLocalNode("chunked", ifaces.NodeOptions(&shellOpts))
	ops := execFileOps{node: node}

	dir := t.TempDir()
	dest := filepath.Join(dir, "large.bin")

	// ~400 KB: several times the ~95 KB one-shot ceiling
	var builder strings.Builder
	for i := 0; i < 8000; i++ {
		builder.WriteString("line of payload content that is repeated many times\n")
	}
	payload := builder.String()
	require.Greater(t, len(payload), 300*1024)

	require.NoError(t, ops.WriteFile(dest, payload, 0644, true, false))

	written, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, len(payload), len(written), "no truncation across chunks")
	assert.Equal(t, payload, string(written))
}

// Binary content with NUL bytes and an empty file both survive.
func TestRemoteWriteBinaryAndEmpty(t *testing.T) {
	shellOpts := map[string]interface{}{"shell": "/bin/sh"}
	node := nodetypes.NewLocalNode("binary", ifaces.NodeOptions(&shellOpts))
	ops := execFileOps{node: node}
	dir := t.TempDir()

	binary := string([]byte{0x00, 0xff, 0x41, 0x00, 0x7f, 0xfe})
	binaryPath := filepath.Join(dir, "bin.dat")
	require.NoError(t, ops.WriteFile(binaryPath, binary, 0644, true, false))
	got, err := os.ReadFile(binaryPath)
	require.NoError(t, err)
	assert.Equal(t, binary, string(got))

	emptyPath := filepath.Join(dir, "empty.dat")
	require.NoError(t, ops.WriteFile(emptyPath, "", 0644, true, false))
	info, err := os.Stat(emptyPath)
	require.NoError(t, err)
	assert.Zero(t, info.Size())
}
