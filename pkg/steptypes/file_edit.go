package steptypes

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &FileEditStep{}

// EditOperation represents the type of edit operation
type EditOperation string

const (
	EditInsert  EditOperation = "insert"
	EditReplace EditOperation = "replace"
	EditRemove  EditOperation = "remove"
)

// InsertPosition represents where to insert content
type InsertPosition string

const (
	InsertBefore InsertPosition = "before"
	InsertAfter  InsertPosition = "after"
)

// MatchType represents the type of matching to use
type MatchType string

const (
	MatchPlain MatchType = "plain"
	MatchRegex MatchType = "regex"
	MatchLine  MatchType = "line"
)

// FileEditStep edits a file using insert, replace, or remove operations.
type FileEditStep struct {
	BaseStep
	filePath  string
	operation EditOperation
	// For insert operations
	position InsertPosition
	// Matching configuration
	matchType  MatchType
	match      string
	lineNumber int
	// Content for insert/replace operations
	content string
	// For regex replace with capture groups
	useCaptures bool
}

// Run executes the file edit operation.
func (s *FileEditStep) Run(updater formatters.TaskCompleter) error {
	// Read the file
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)
	var result string

	switch s.operation {
	case EditInsert:
		result, err = s.doInsert(content)
	case EditReplace:
		result, err = s.doReplace(content)
	case EditRemove:
		result, err = s.doRemove(content)
	default:
		updater.Error()
		return fmt.Errorf("unknown edit operation: %s", s.operation)
	}

	if err != nil {
		updater.Error()
		return err
	}

	// Write the modified content back
	err = os.WriteFile(s.filePath, []byte(result), 0644)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to write file: %w", err)
	}

	updater.Complete()
	return nil
}

// doInsert inserts content before or after a match
func (s *FileEditStep) doInsert(content string) (string, error) {
	switch s.matchType {
	case MatchLine:
		return s.insertByLine(content)
	case MatchPlain:
		return s.insertByPlainMatch(content)
	case MatchRegex:
		return s.insertByRegexMatch(content)
	default:
		return "", fmt.Errorf("unknown match type: %s", s.matchType)
	}
}

// insertByLine inserts content before or after a specific line number
func (s *FileEditStep) insertByLine(content string) (string, error) {
	lines := strings.Split(content, "\n")
	if s.lineNumber < 1 || s.lineNumber > len(lines) {
		return "", fmt.Errorf("line number %d is out of range (1-%d)", s.lineNumber, len(lines))
	}

	idx := s.lineNumber - 1 // Convert to 0-based index
	insertLines := strings.Split(s.content, "\n")

	var result []string
	if s.position == InsertBefore {
		result = append(result, lines[:idx]...)
		result = append(result, insertLines...)
		result = append(result, lines[idx:]...)
	} else { // InsertAfter
		result = append(result, lines[:idx+1]...)
		result = append(result, insertLines...)
		result = append(result, lines[idx+1:]...)
	}

	return strings.Join(result, "\n"), nil
}

// insertByPlainMatch inserts content before or after a plain text match
func (s *FileEditStep) insertByPlainMatch(content string) (string, error) {
	idx := strings.Index(content, s.match)
	if idx == -1 {
		return "", fmt.Errorf("match not found: %s", s.match)
	}

	if s.position == InsertBefore {
		return content[:idx] + s.content + content[idx:], nil
	}
	// InsertAfter
	endIdx := idx + len(s.match)
	return content[:endIdx] + s.content + content[endIdx:], nil
}

// insertByRegexMatch inserts content before or after a regex match
func (s *FileEditStep) insertByRegexMatch(content string) (string, error) {
	re, err := regexp.Compile(s.match)
	if err != nil {
		return "", fmt.Errorf("invalid regex: %w", err)
	}

	loc := re.FindStringIndex(content)
	if loc == nil {
		return "", fmt.Errorf("regex match not found: %s", s.match)
	}

	if s.position == InsertBefore {
		return content[:loc[0]] + s.content + content[loc[0]:], nil
	}
	// InsertAfter
	return content[:loc[1]] + s.content + content[loc[1]:], nil
}

// doReplace replaces matched content
func (s *FileEditStep) doReplace(content string) (string, error) {
	switch s.matchType {
	case MatchPlain:
		return s.replaceByPlainMatch(content)
	case MatchRegex:
		return s.replaceByRegexMatch(content)
	default:
		return "", fmt.Errorf("unsupported match type for replace: %s", s.matchType)
	}
}

// replaceByPlainMatch replaces plain text matches
func (s *FileEditStep) replaceByPlainMatch(content string) (string, error) {
	if !strings.Contains(content, s.match) {
		return "", fmt.Errorf("match not found: %s", s.match)
	}
	return strings.ReplaceAll(content, s.match, s.content), nil
}

// replaceByRegexMatch replaces regex matches, optionally using capture groups
func (s *FileEditStep) replaceByRegexMatch(content string) (string, error) {
	re, err := regexp.Compile(s.match)
	if err != nil {
		return "", fmt.Errorf("invalid regex: %w", err)
	}

	if !re.MatchString(content) {
		return "", fmt.Errorf("regex match not found: %s", s.match)
	}

	if s.useCaptures {
		return s.replaceWithCaptures(content, re)
	}

	return re.ReplaceAllString(content, s.content), nil
}

// replaceWithCaptures handles replacement with capture group references
// Supports $1, $2, etc. or ${1}, ${name} syntax in the replacement string
func (s *FileEditStep) replaceWithCaptures(content string, re *regexp.Regexp) (string, error) {
	result := re.ReplaceAllStringFunc(content, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		replacement := s.content

		// Replace named groups first ${name}
		for i, name := range re.SubexpNames() {
			if name != "" && i < len(submatches) {
				replacement = strings.ReplaceAll(replacement, "${"+name+"}", submatches[i])
			}
		}

		// Replace numbered groups $1, $2, etc. or ${1}, ${2}, etc.
		for i := len(submatches) - 1; i >= 0; i-- {
			placeholder := "$" + strconv.Itoa(i)
			bracedPlaceholder := "${" + strconv.Itoa(i) + "}"
			replacement = strings.ReplaceAll(replacement, bracedPlaceholder, submatches[i])
			replacement = strings.ReplaceAll(replacement, placeholder, submatches[i])
		}

		return replacement
	})

	return result, nil
}

// doRemove removes matched content
func (s *FileEditStep) doRemove(content string) (string, error) {
	switch s.matchType {
	case MatchPlain:
		return s.removeByPlainMatch(content)
	case MatchRegex:
		return s.removeByRegexMatch(content)
	default:
		return "", fmt.Errorf("unsupported match type for remove: %s", s.matchType)
	}
}

// removeByPlainMatch removes plain text matches
func (s *FileEditStep) removeByPlainMatch(content string) (string, error) {
	if !strings.Contains(content, s.match) {
		return "", fmt.Errorf("match not found: %s", s.match)
	}
	return strings.ReplaceAll(content, s.match, ""), nil
}

// removeByRegexMatch removes regex matches
func (s *FileEditStep) removeByRegexMatch(content string) (string, error) {
	re, err := regexp.Compile(s.match)
	if err != nil {
		return "", fmt.Errorf("invalid regex: %w", err)
	}

	if !re.MatchString(content) {
		return "", fmt.Errorf("regex match not found: %s", s.match)
	}

	return re.ReplaceAllString(content, ""), nil
}
