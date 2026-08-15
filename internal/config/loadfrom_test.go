package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fragments are spliced into an enclosing document and then indented, so a
// document separator would land as an indented `---` mid-document. Every
// suite example in this repository opens with one, so copying an example into
// a load_from directory hit this immediately.
func TestStripDocumentMarkers(t *testing.T) {
	cases := map[string]struct{ in, want string }{
		"leading separator": {
			in:   "---\n- name: local\n  type: local\n",
			want: "- name: local\n  type: local\n",
		},
		"separator after comments": {
			in:   "# a note\n---\n- name: local\n",
			want: "# a note\n- name: local\n",
		},
		"trailing end marker": {
			in:   "- name: local\n...\n",
			want: "- name: local\n",
		},
		"both markers": {
			in:   "---\n- name: local\n...\n",
			want: "- name: local\n",
		},
		"no markers": {
			in:   "- name: local\n  type: local\n",
			want: "- name: local\n  type: local\n",
		},
		// A --- inside a block scalar is content, not a separator, and must
		// survive: only a marker at the document's own level is removed
		"separator inside content": {
			in:   "- name: local\n  script: |\n    ---\n    still content\n",
			want: "- name: local\n  script: |\n    ---\n    still content\n",
		},
		"value that merely contains dashes": {
			in:   "- name: a---b\n",
			want: "- name: a---b\n",
		},
	}
	for name, tc := range cases {
		assert.Equal(t, tc.want, string(stripDocumentMarkers([]byte(tc.in))), name)
	}
}

// A fragment carrying a separator must load, since that is what people
// naturally write.
func TestLoadFromToleratesDocumentSeparators(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "nodes"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.yaml"), []byte(`suite: Demo
nodes: !!load_from(nodes)
tests: []
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nodes", "nodes.yaml"), []byte(`---
- name: local
  type: local
`), 0o644))

	cfg, err := LoadConfiguration(filepath.Join(dir, "main.yaml"))
	require.NoError(t, err)
	require.Len(t, cfg.Nodes, 1)
	assert.Equal(t, "local", cfg.Nodes[0].Name)
}

// When fragments are spliced the reported line refers to the assembled
// document, so the error names the files that went into it.
func TestLoadFromErrorNamesFragments(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "nodes"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.yaml"), []byte(`suite: Demo
nodes: !!load_from(nodes)
tests: []
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nodes", "broken.yaml"), []byte(`- name: local
  type: local
   bad: indent
`), 0o644))

	_, err := LoadConfiguration(filepath.Join(dir, "main.yaml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "!!load_from")
	assert.Contains(t, err.Error(), "broken.yaml", "the error must say which fragments were spliced")
}
