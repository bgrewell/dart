package steptypes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileEditStepInsertAfterLine verifies inserting after a line number.
func TestFileEditStepInsertAfterLine(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_edit_insert_line.txt")
	defer os.Remove(tempFile)

	err := os.WriteFile(tempFile, []byte("line 1\nline 2\nline 3"), 0644)
	require.NoError(t, err)

	step := &FileEditStep{
		BaseStep:   BaseStep{title: "Insert After Line"},
		filePath:   tempFile,
		operation:  EditInsert,
		position:   InsertAfter,
		matchType:  MatchLine,
		lineNumber: 2,
		content:    "inserted line",
	}

	updater := formatters.NewMockTaskCompleter()
	err = step.Run(updater)

	assert.NoError(t, err)
	assert.True(t, updater.IsCompleted())

	content, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.Equal(t, "line 1\nline 2\ninserted line\nline 3", string(content))
}

// TestFileEditStepInsertBeforeLine verifies inserting before a line number.
func TestFileEditStepInsertBeforeLine(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_edit_insert_before_line.txt")
	defer os.Remove(tempFile)

	err := os.WriteFile(tempFile, []byte("line 1\nline 2\nline 3"), 0644)
	require.NoError(t, err)

	step := &FileEditStep{
		BaseStep:   BaseStep{title: "Insert Before Line"},
		filePath:   tempFile,
		operation:  EditInsert,
		position:   InsertBefore,
		matchType:  MatchLine,
		lineNumber: 2,
		content:    "inserted line",
	}

	updater := formatters.NewMockTaskCompleter()
	err = step.Run(updater)

	assert.NoError(t, err)

	content, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.Equal(t, "line 1\ninserted line\nline 2\nline 3", string(content))
}

// TestFileEditStepInsertByPlainMatch verifies inserting after a plain text match.
func TestFileEditStepInsertByPlainMatch(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_edit_insert_plain.txt")
	defer os.Remove(tempFile)

	err := os.WriteFile(tempFile, []byte("Hello World"), 0644)
	require.NoError(t, err)

	step := &FileEditStep{
		BaseStep:  BaseStep{title: "Insert After Plain"},
		filePath:  tempFile,
		operation: EditInsert,
		position:  InsertAfter,
		matchType: MatchPlain,
		match:     "Hello",
		content:   " Beautiful",
	}

	updater := formatters.NewMockTaskCompleter()
	err = step.Run(updater)

	assert.NoError(t, err)

	content, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.Equal(t, "Hello Beautiful World", string(content))
}

// TestFileEditStepInsertByRegexMatch verifies inserting after a regex match.
func TestFileEditStepInsertByRegexMatch(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_edit_insert_regex.txt")
	defer os.Remove(tempFile)

	err := os.WriteFile(tempFile, []byte("Version: 1.2.3"), 0644)
	require.NoError(t, err)

	step := &FileEditStep{
		BaseStep:  BaseStep{title: "Insert After Regex"},
		filePath:  tempFile,
		operation: EditInsert,
		position:  InsertAfter,
		matchType: MatchRegex,
		match:     `\d+\.\d+\.\d+`,
		content:   "-beta",
	}

	updater := formatters.NewMockTaskCompleter()
	err = step.Run(updater)

	assert.NoError(t, err)

	content, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.Equal(t, "Version: 1.2.3-beta", string(content))
}

// TestFileEditStepReplacePlain verifies plain text replacement.
func TestFileEditStepReplacePlain(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_edit_replace_plain.txt")
	defer os.Remove(tempFile)

	err := os.WriteFile(tempFile, []byte("Hello World, Hello Universe"), 0644)
	require.NoError(t, err)

	step := &FileEditStep{
		BaseStep:  BaseStep{title: "Replace Plain"},
		filePath:  tempFile,
		operation: EditReplace,
		matchType: MatchPlain,
		match:     "Hello",
		content:   "Hi",
	}

	updater := formatters.NewMockTaskCompleter()
	err = step.Run(updater)

	assert.NoError(t, err)

	content, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.Equal(t, "Hi World, Hi Universe", string(content))
}

// TestFileEditStepReplaceRegex verifies regex replacement.
func TestFileEditStepReplaceRegex(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_edit_replace_regex.txt")
	defer os.Remove(tempFile)

	err := os.WriteFile(tempFile, []byte("Version: 1.2.3, Build: 4.5.6"), 0644)
	require.NoError(t, err)

	step := &FileEditStep{
		BaseStep:  BaseStep{title: "Replace Regex"},
		filePath:  tempFile,
		operation: EditReplace,
		matchType: MatchRegex,
		match:     `\d+\.\d+\.\d+`,
		content:   "X.X.X",
	}

	updater := formatters.NewMockTaskCompleter()
	err = step.Run(updater)

	assert.NoError(t, err)

	content, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.Equal(t, "Version: X.X.X, Build: X.X.X", string(content))
}

// TestFileEditStepReplaceWithCaptures verifies regex replacement with capture groups.
func TestFileEditStepReplaceWithCaptures(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_edit_replace_captures.txt")
	defer os.Remove(tempFile)

	err := os.WriteFile(tempFile, []byte("name: John, age: 30"), 0644)
	require.NoError(t, err)

	step := &FileEditStep{
		BaseStep:    BaseStep{title: "Replace With Captures"},
		filePath:    tempFile,
		operation:   EditReplace,
		matchType:   MatchRegex,
		match:       `name: (\w+), age: (\d+)`,
		content:     "person: $1 ($2 years old)",
		useCaptures: true,
	}

	updater := formatters.NewMockTaskCompleter()
	err = step.Run(updater)

	assert.NoError(t, err)

	content, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.Equal(t, "person: John (30 years old)", string(content))
}

// TestFileEditStepReplaceWithNamedCaptures verifies regex replacement with named capture groups.
func TestFileEditStepReplaceWithNamedCaptures(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_edit_replace_named_captures.txt")
	defer os.Remove(tempFile)

	err := os.WriteFile(tempFile, []byte("name: John, age: 30"), 0644)
	require.NoError(t, err)

	step := &FileEditStep{
		BaseStep:    BaseStep{title: "Replace With Named Captures"},
		filePath:    tempFile,
		operation:   EditReplace,
		matchType:   MatchRegex,
		match:       `name: (?P<name>\w+), age: (?P<age>\d+)`,
		content:     "person: ${name} (${age} years old)",
		useCaptures: true,
	}

	updater := formatters.NewMockTaskCompleter()
	err = step.Run(updater)

	assert.NoError(t, err)

	content, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.Equal(t, "person: John (30 years old)", string(content))
}

// TestFileEditStepRemovePlain verifies plain text removal.
func TestFileEditStepRemovePlain(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_edit_remove_plain.txt")
	defer os.Remove(tempFile)

	err := os.WriteFile(tempFile, []byte("Hello World, Hello Universe"), 0644)
	require.NoError(t, err)

	step := &FileEditStep{
		BaseStep:  BaseStep{title: "Remove Plain"},
		filePath:  tempFile,
		operation: EditRemove,
		matchType: MatchPlain,
		match:     "Hello ",
	}

	updater := formatters.NewMockTaskCompleter()
	err = step.Run(updater)

	assert.NoError(t, err)

	content, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.Equal(t, "World, Universe", string(content))
}

// TestFileEditStepRemoveRegex verifies regex removal.
func TestFileEditStepRemoveRegex(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_edit_remove_regex.txt")
	defer os.Remove(tempFile)

	err := os.WriteFile(tempFile, []byte("Item 1, Item 2, Item 3"), 0644)
	require.NoError(t, err)

	step := &FileEditStep{
		BaseStep:  BaseStep{title: "Remove Regex"},
		filePath:  tempFile,
		operation: EditRemove,
		matchType: MatchRegex,
		match:     `, Item \d+`,
	}

	updater := formatters.NewMockTaskCompleter()
	err = step.Run(updater)

	assert.NoError(t, err)

	content, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.Equal(t, "Item 1", string(content))
}

// TestFileEditStepMatchNotFound verifies error when match is not found.
func TestFileEditStepMatchNotFound(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_edit_match_not_found.txt")
	defer os.Remove(tempFile)

	err := os.WriteFile(tempFile, []byte("Hello World"), 0644)
	require.NoError(t, err)

	step := &FileEditStep{
		BaseStep:  BaseStep{title: "Match Not Found"},
		filePath:  tempFile,
		operation: EditReplace,
		matchType: MatchPlain,
		match:     "NotInFile",
		content:   "Replacement",
	}

	updater := formatters.NewMockTaskCompleter()
	err = step.Run(updater)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "match not found")
	assert.True(t, updater.IsErrored())
}

// TestFileEditStepInvalidRegex verifies error handling for invalid regex.
func TestFileEditStepInvalidRegex(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_edit_invalid_regex.txt")
	defer os.Remove(tempFile)

	err := os.WriteFile(tempFile, []byte("Hello World"), 0644)
	require.NoError(t, err)

	step := &FileEditStep{
		BaseStep:  BaseStep{title: "Invalid Regex"},
		filePath:  tempFile,
		operation: EditReplace,
		matchType: MatchRegex,
		match:     "[invalid",
		content:   "Replacement",
	}

	updater := formatters.NewMockTaskCompleter()
	err = step.Run(updater)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regex")
	assert.True(t, updater.IsErrored())
}

// TestFileEditStepLineNumberOutOfRange verifies error for out-of-range line number.
func TestFileEditStepLineNumberOutOfRange(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_edit_line_range.txt")
	defer os.Remove(tempFile)

	err := os.WriteFile(tempFile, []byte("line 1\nline 2"), 0644)
	require.NoError(t, err)

	step := &FileEditStep{
		BaseStep:   BaseStep{title: "Line Out of Range"},
		filePath:   tempFile,
		operation:  EditInsert,
		matchType:  MatchLine,
		lineNumber: 10,
		content:    "new line",
	}

	updater := formatters.NewMockTaskCompleter()
	err = step.Run(updater)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
	assert.True(t, updater.IsErrored())
}
