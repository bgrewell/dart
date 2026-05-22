package steptypes

import (
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runEdit seeds a MockNode with initial, runs the step, and returns the resulting file contents.
func runEdit(t *testing.T, path, initial string, step *FileEditStep) (string, error) {
	t.Helper()
	node := nodetypes.NewMockNode()
	node.SeedFile(path, []byte(initial), 0644)
	step.node = node
	step.filePath = path

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)
	if err != nil {
		return "", err
	}
	got, ok := node.GetFile(path)
	require.True(t, ok, "expected file to exist after edit")
	return string(got), nil
}

// TestFileEditStepInsertAfterLine verifies inserting after a line number.
func TestFileEditStepInsertAfterLine(t *testing.T) {
	got, err := runEdit(t, "/etc/edit_insert_line.txt", "line 1\nline 2\nline 3", &FileEditStep{
		BaseStep:   BaseStep{title: "Insert After Line"},
		operation:  EditInsert,
		position:   InsertAfter,
		matchType:  MatchLine,
		lineNumber: 2,
		content:    "inserted line",
	})
	assert.NoError(t, err)
	assert.Equal(t, "line 1\nline 2\ninserted line\nline 3", got)
}

// TestFileEditStepInsertBeforeLine verifies inserting before a line number.
func TestFileEditStepInsertBeforeLine(t *testing.T) {
	got, err := runEdit(t, "/etc/edit_insert_before_line.txt", "line 1\nline 2\nline 3", &FileEditStep{
		BaseStep:   BaseStep{title: "Insert Before Line"},
		operation:  EditInsert,
		position:   InsertBefore,
		matchType:  MatchLine,
		lineNumber: 2,
		content:    "inserted line",
	})
	assert.NoError(t, err)
	assert.Equal(t, "line 1\ninserted line\nline 2\nline 3", got)
}

// TestFileEditStepInsertByPlainMatch verifies inserting after a plain text match.
func TestFileEditStepInsertByPlainMatch(t *testing.T) {
	got, err := runEdit(t, "/etc/edit_insert_plain.txt", "Hello World", &FileEditStep{
		BaseStep:  BaseStep{title: "Insert After Plain"},
		operation: EditInsert,
		position:  InsertAfter,
		matchType: MatchPlain,
		match:     "Hello",
		content:   " Beautiful",
	})
	assert.NoError(t, err)
	assert.Equal(t, "Hello Beautiful World", got)
}

// TestFileEditStepInsertByRegexMatch verifies inserting after a regex match.
func TestFileEditStepInsertByRegexMatch(t *testing.T) {
	got, err := runEdit(t, "/etc/edit_insert_regex.txt", "Version: 1.2.3", &FileEditStep{
		BaseStep:  BaseStep{title: "Insert After Regex"},
		operation: EditInsert,
		position:  InsertAfter,
		matchType: MatchRegex,
		match:     `\d+\.\d+\.\d+`,
		content:   "-beta",
	})
	assert.NoError(t, err)
	assert.Equal(t, "Version: 1.2.3-beta", got)
}

// TestFileEditStepReplacePlain verifies plain text replacement.
func TestFileEditStepReplacePlain(t *testing.T) {
	got, err := runEdit(t, "/etc/edit_replace_plain.txt", "Hello World, Hello Universe", &FileEditStep{
		BaseStep:  BaseStep{title: "Replace Plain"},
		operation: EditReplace,
		matchType: MatchPlain,
		match:     "Hello",
		content:   "Hi",
	})
	assert.NoError(t, err)
	assert.Equal(t, "Hi World, Hi Universe", got)
}

// TestFileEditStepReplaceRegex verifies regex replacement.
func TestFileEditStepReplaceRegex(t *testing.T) {
	got, err := runEdit(t, "/etc/edit_replace_regex.txt", "Version: 1.2.3, Build: 4.5.6", &FileEditStep{
		BaseStep:  BaseStep{title: "Replace Regex"},
		operation: EditReplace,
		matchType: MatchRegex,
		match:     `\d+\.\d+\.\d+`,
		content:   "X.X.X",
	})
	assert.NoError(t, err)
	assert.Equal(t, "Version: X.X.X, Build: X.X.X", got)
}

// TestFileEditStepReplaceWithCaptures verifies regex replacement with capture groups.
func TestFileEditStepReplaceWithCaptures(t *testing.T) {
	got, err := runEdit(t, "/etc/edit_replace_captures.txt", "name: John, age: 30", &FileEditStep{
		BaseStep:    BaseStep{title: "Replace With Captures"},
		operation:   EditReplace,
		matchType:   MatchRegex,
		match:       `name: (\w+), age: (\d+)`,
		content:     "person: $1 ($2 years old)",
		useCaptures: true,
	})
	assert.NoError(t, err)
	assert.Equal(t, "person: John (30 years old)", got)
}

// TestFileEditStepReplaceWithNamedCaptures verifies regex replacement with named capture groups.
func TestFileEditStepReplaceWithNamedCaptures(t *testing.T) {
	got, err := runEdit(t, "/etc/edit_replace_named.txt", "name: John, age: 30", &FileEditStep{
		BaseStep:    BaseStep{title: "Replace With Named Captures"},
		operation:   EditReplace,
		matchType:   MatchRegex,
		match:       `name: (?P<name>\w+), age: (?P<age>\d+)`,
		content:     "person: ${name} (${age} years old)",
		useCaptures: true,
	})
	assert.NoError(t, err)
	assert.Equal(t, "person: John (30 years old)", got)
}

// TestFileEditStepRemovePlain verifies plain text removal.
func TestFileEditStepRemovePlain(t *testing.T) {
	got, err := runEdit(t, "/etc/edit_remove_plain.txt", "Hello World, Hello Universe", &FileEditStep{
		BaseStep:  BaseStep{title: "Remove Plain"},
		operation: EditRemove,
		matchType: MatchPlain,
		match:     "Hello ",
	})
	assert.NoError(t, err)
	assert.Equal(t, "World, Universe", got)
}

// TestFileEditStepRemoveRegex verifies regex removal.
func TestFileEditStepRemoveRegex(t *testing.T) {
	got, err := runEdit(t, "/etc/edit_remove_regex.txt", "Item 1, Item 2, Item 3", &FileEditStep{
		BaseStep:  BaseStep{title: "Remove Regex"},
		operation: EditRemove,
		matchType: MatchRegex,
		match:     `, Item \d+`,
	})
	assert.NoError(t, err)
	assert.Equal(t, "Item 1", got)
}

// TestFileEditStepMatchNotFound verifies error when match is not found.
func TestFileEditStepMatchNotFound(t *testing.T) {
	_, err := runEdit(t, "/etc/edit_match_not_found.txt", "Hello World", &FileEditStep{
		BaseStep:  BaseStep{title: "Match Not Found"},
		operation: EditReplace,
		matchType: MatchPlain,
		match:     "NotInFile",
		content:   "Replacement",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "match not found")
}

// TestFileEditStepInvalidRegex verifies error handling for invalid regex.
func TestFileEditStepInvalidRegex(t *testing.T) {
	_, err := runEdit(t, "/etc/edit_invalid_regex.txt", "Hello World", &FileEditStep{
		BaseStep:  BaseStep{title: "Invalid Regex"},
		operation: EditReplace,
		matchType: MatchRegex,
		match:     "[invalid",
		content:   "Replacement",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regex")
}

// TestFileEditStepLineNumberOutOfRange verifies error for out-of-range line number.
func TestFileEditStepLineNumberOutOfRange(t *testing.T) {
	_, err := runEdit(t, "/etc/edit_line_range.txt", "line 1\nline 2", &FileEditStep{
		BaseStep:   BaseStep{title: "Line Out of Range"},
		operation:  EditInsert,
		matchType:  MatchLine,
		lineNumber: 10,
		content:    "new line",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}
