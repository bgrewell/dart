package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

// SourceLocation points to a specific position in a YAML configuration file.
type SourceLocation struct {
	File   string
	Line   int
	Column int
}

// ConfigError represents a configuration validation error tied to a specific
// location in a YAML file. Factory functions in nodetypes/steptypes/testtypes
// return this type so that the top-level error handler can render a contextual
// snippet showing the offending line.
type ConfigError struct {
	Message  string
	Location SourceLocation
	Key      string
}

func (e *ConfigError) Error() string {
	return e.Message
}

// RenderConfigError reads the YAML file referenced by the error's location and
// returns a colored snippet with the offending line highlighted. Shows 3 lines
// of context above and 2 lines below.
func RenderConfigError(cfgErr *ConfigError) string {
	var b strings.Builder

	red := color.New(color.FgRed, color.Bold)
	cyan := color.New(color.FgCyan)
	dim := color.New(color.Faint)
	redLine := color.New(color.FgRed)

	b.WriteString("\n")
	b.WriteString(red.Sprintf("Error: %s", cfgErr.Message))
	b.WriteString("\n")

	if cfgErr.Location.File == "" || cfgErr.Location.Line == 0 {
		b.WriteString("\n")
		return b.String()
	}

	data, err := os.ReadFile(cfgErr.Location.File)
	if err != nil {
		// Can't read the file; fall back to showing just file:line
		b.WriteString(fmt.Sprintf("\n  %s:%d\n\n", cfgErr.Location.File, cfgErr.Location.Line))
		return b.String()
	}

	lines := strings.Split(string(data), "\n")
	targetLine := cfgErr.Location.Line // 1-based

	// Context window: 3 above, 2 below
	startLine := targetLine - 3
	if startLine < 1 {
		startLine = 1
	}
	endLine := targetLine + 2
	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Compute width for line number gutter
	gutterWidth := len(fmt.Sprintf("%d", endLine))

	b.WriteString("\n")
	b.WriteString(cyan.Sprintf("  %s", cfgErr.Location.File))
	b.WriteString("\n\n")

	separator := strings.Repeat("\u2500", 37)
	b.WriteString(dim.Sprintf("  %s", separator))
	b.WriteString("\n")

	for lineNum := startLine; lineNum <= endLine; lineNum++ {
		lineContent := ""
		if lineNum <= len(lines) {
			lineContent = lines[lineNum-1]
		}

		numStr := fmt.Sprintf("%*d", gutterWidth, lineNum)

		if lineNum == targetLine {
			b.WriteString(redLine.Sprintf("  > %s | %s", numStr, lineContent))
		} else {
			b.WriteString(fmt.Sprintf("    %s | %s", numStr, lineContent))
		}
		b.WriteString("\n")
	}

	b.WriteString(dim.Sprintf("  %s", separator))
	b.WriteString("\n\n")

	return b.String()
}
