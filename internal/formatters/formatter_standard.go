package formatters

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bgrewell/dart/internal/results"
	"github.com/bgrewell/dart/internal/stream"
	"github.com/fatih/color"
	"github.com/theckman/yacspin"
)

var _ Formatter = &StandardFormatter{}

var (
	headerColor        = color.New(color.FgHiBlue).Add(color.Bold)
	headerPrefixColor  = color.New(color.FgHiWhite).Add(color.Bold)
	headerFailColor    = color.New(color.FgHiRed).Add(color.Bold)
	headerPassColor    = color.New(color.FgHiGreen).Add(color.Bold)
	numberColor        = color.New(color.FgHiCyan)
	numberPaddingColor = color.New(color.FgHiBlack)
	labelFailColor     = color.New(color.FgHiRed)
	valueColor         = color.New(color.FgHiCyan)
	valuePassColor     = color.New(color.FgHiGreen)
	valueFailColor     = color.New(color.FgHiRed)
	valueRanColor      = color.New(color.FgHiYellow)
	nodeNameColor      = color.New(color.FgHiGreen)
	nodeBracketColor   = numberPaddingColor
)

func NewStandardFormatter() *StandardFormatter {
	return &StandardFormatter{
		indent:       2,
		detailIndent: 7,
		out:          os.Stdout,
	}
}

// NewStandardFormatterWithWriter directs all formatter output (including
// spinner output) to w — for tests and captured runs.
func NewStandardFormatterWithWriter(w io.Writer) *StandardFormatter {
	sf := NewStandardFormatter()
	sf.out = w
	return sf
}

type StandardFormatter struct {
	taskColumnWidth int
	testColumnWidth int
	nodeNameWidth   int
	indent          int
	detailIndent    int
	out             io.Writer
}

func (sf *StandardFormatter) PrintError(err error) {
	fmt.Fprintf(sf.out, "%s%s\n", strings.Repeat(" ", sf.detailIndent-sf.indent), valueFailColor.Sprint(err.Error()))
}

func (sf *StandardFormatter) PrintPass(name string, details interface{}) {
	fmt.Fprintf(sf.out, "%s+%s:\n", strings.Repeat(" ", sf.detailIndent-sf.indent), headerPassColor.Sprint(name))
	sf.printDetails(details)
}

func (sf *StandardFormatter) PrintFail(name string, details interface{}) {
	fmt.Fprintf(sf.out, "%s-%s:\n", strings.Repeat(" ", sf.detailIndent-sf.indent), headerFailColor.Sprint(name))
	switch d := details.(type) {
	case *results.ResultStringMatchFail:
		fmt.Fprintf(sf.out, "%s%s: %s\n", strings.Repeat(" ", sf.detailIndent), labelFailColor.Sprint("Expected"), d.Expected)
		fmt.Fprintf(sf.out, "%s%s: %s\n", strings.Repeat(" ", sf.detailIndent), labelFailColor.Sprint("Actual"), d.Actual)
	case *results.ResultIntMatchFail:
		fmt.Fprintf(sf.out, "%s%s: %d\n", strings.Repeat(" ", sf.detailIndent), labelFailColor.Sprint("Expected"), d.Expected)
		fmt.Fprintf(sf.out, "%s%s: %d\n", strings.Repeat(" ", sf.detailIndent), labelFailColor.Sprint("Actual"), d.Actual)
	default:
		sf.printDetails(details)
	}
}

// printDetails renders a detail value of any type; unknown types render via
// fmt.Sprint rather than printing nothing.
func (sf *StandardFormatter) printDetails(details interface{}) {
	if details == nil {
		return
	}
	var text string
	switch d := details.(type) {
	case string:
		text = d
	case int:
		text = strconv.Itoa(d)
	default:
		text = fmt.Sprint(d)
	}
	for _, line := range strings.Split(text, "\n") {
		fmt.Fprintf(sf.out, "%s%s\n", strings.Repeat(" ", sf.detailIndent), valueColor.Sprint(line))
	}
}

func (sf *StandardFormatter) PrintEmpty() {
	fmt.Fprintln(sf.out)
}

func (sf *StandardFormatter) PrintResults(pass, fail, skipped, ran int) {

	p := 5 - (len(strconv.Itoa(pass)))
	f := 5 - (len(strconv.Itoa(fail)))
	s := 5 - (len(strconv.Itoa(skipped)))
	r := 5 - (len(strconv.Itoa(ran)))

	passVal := strconv.Itoa(pass)
	failVal := strconv.Itoa(fail)
	skipVal := strconv.Itoa(skipped)
	ranVal := strconv.Itoa(ran)

	if pass == 0 {
		p = 5
		passVal = ""
	}
	if fail == 0 {
		f = 5
		failVal = ""
	}

	passPad := strings.Repeat("0", p)
	failPad := strings.Repeat("0", f)
	skipPad := strings.Repeat("0", s)
	ranPad := strings.Repeat("0", r)

	indent := strings.Repeat(" ", sf.indent)
	sf.PrintHeader("Results")
	fmt.Fprintf(sf.out, "%sPass: %s%s\n", indent, numberPaddingColor.Sprint(passPad), valuePassColor.Sprint(passVal))
	fmt.Fprintf(sf.out, "%sFail: %s%s\n", indent, numberPaddingColor.Sprint(failPad), valueFailColor.Sprint(failVal))
	if skipped > 0 {
		fmt.Fprintf(sf.out, "%sSkip: %s%s\n", indent, numberPaddingColor.Sprint(skipPad), valueRanColor.Sprint(skipVal))
	}
	if ran > 0 {
		fmt.Fprintf(sf.out, "%sRan:  %s%s\n", indent, numberPaddingColor.Sprint(ranPad), valueRanColor.Sprint(ranVal))

	}
}

// PrintSkip reports a skipped test with the reason its condition triggered.
func (sf *StandardFormatter) PrintSkip(name string, reason string) {
	fmt.Fprintf(sf.out, "%s~%s:\n", strings.Repeat(" ", sf.detailIndent-sf.indent), valueRanColor.Sprint(name))
	fmt.Fprintf(sf.out, "%s%s\n", strings.Repeat(" ", sf.detailIndent), valueColor.Sprint(reason))
}

func (sf *StandardFormatter) PrintHeader(header string) {
	fmt.Fprintf(sf.out, "%s%s\n", headerPrefixColor.Sprint("[+] "), headerColor.Sprint(header))
}

func (sf *StandardFormatter) SetTaskColumnWidth(width int) {
	sf.taskColumnWidth = width
}

func (sf *StandardFormatter) SetTestColumnWidth(width int) {
	sf.testColumnWidth = width
}

func (sf *StandardFormatter) SetNodeNameWidth(width int) {
	sf.nodeNameWidth = width
}

func (sf *StandardFormatter) StartTask(task, nodeName, status string) TaskCompleter {

	spinner, _ := yacspin.New(yacspin.Config{
		Frequency:         100 * time.Millisecond,
		Writer:            sf.out,
		ShowCursor:        false,
		SpinnerAtEnd:      true,
		CharSet:           yacspin.CharSets[14],
		Colors:            []string{"fgHiCyan"},
		StopColors:        []string{"fgHiGreen"},
		StopFailColors:    []string{"fgHiRed"},
		StopFailCharacter: "error", //"✗",
		StopCharacter:     "done",  //"✓",
	})
	c := &StandardTaskCompleter{
		BaseCompleter: BaseCompleter{
			spinner: spinner,
		},
		Message: padRightWithPeriods(task, sf.taskColumnWidth-len(task)+3),
	}

	indent := strings.Repeat(" ", sf.indent)
	nodeBox := sf.formatNodeBox(nodeName)
	message := fmt.Sprintf("%s%s%s", indent, nodeBox, c.Message)
	messages := []func(string){c.spinner.Message, c.spinner.StopMessage, c.spinner.StopFailMessage}
	c.spinner.Start()
	for _, m := range messages {
		m(message)
	}

	// Register spinner with coordinator for debug output coordination
	stream.GetCoordinator().SetActiveSpinner(c.spinner)

	return c
}

func (sf *StandardFormatter) StartTest(id, name, nodeName string) TestCompleter {
	spinner, _ := yacspin.New(yacspin.Config{
		Frequency:         100 * time.Millisecond,
		Writer:            sf.out,
		ShowCursor:        false,
		SpinnerAtEnd:      true,
		CharSet:           yacspin.CharSets[14],
		Colors:            []string{"fgHiCyan"},
		StopColors:        []string{"fgHiGreen"},
		StopFailColors:    []string{"fgHiRed"},
		StopFailCharacter: "failed", //"✗",
		StopCharacter:     "passed", //"✓",
	})

	c := &StandardTestCompleter{
		BaseCompleter: BaseCompleter{
			spinner: spinner,
		},
		TestId:   id,
		TestName: padRightWithPeriods(name, sf.testColumnWidth-len(name)+3),
	}

	padWidth := 5 - len(id)
	if padWidth < 0 {
		padWidth = 0
	}
	pad := strings.Repeat("0", padWidth)
	indent := strings.Repeat(" ", sf.indent)
	nodeBox := sf.formatNodeBox(nodeName)
	message := fmt.Sprintf("%s%s%s: %s%s", indent, numberPaddingColor.Sprint(pad), numberColor.Sprint(c.TestId), nodeBox, c.TestName)
	messages := []func(string){c.spinner.Message, c.spinner.StopMessage, c.spinner.StopFailMessage}
	c.spinner.Start()
	for _, m := range messages {
		m(message)
	}

	// Register spinner with coordinator for debug output coordination
	stream.GetCoordinator().SetActiveSpinner(c.spinner)

	return c
}

type StandardTaskCompleter struct {
	BaseCompleter
	Message string
}

func (s StandardTaskCompleter) Update(status string) {
	//s.spinner.Message(status)
}

func (s StandardTaskCompleter) Complete() {
	stream.GetCoordinator().ClearActiveSpinner()
	s.spinner.Stop()
}

func (s StandardTaskCompleter) Fail() {
	stream.GetCoordinator().ClearActiveSpinner()
	s.spinner.StopFail()
}

func (s StandardTaskCompleter) Error() {
	stream.GetCoordinator().ClearActiveSpinner()
	s.spinner.StopFailCharacter("error")
	s.spinner.StopFail()
}

type StandardTestCompleter struct {
	BaseCompleter
	TestId   string
	TestName string
}

func (s StandardTestCompleter) Update(status string) {
	//s.spinner.Message(status)
}

func (s StandardTestCompleter) Complete(passed []bool) {
	stream.GetCoordinator().ClearActiveSpinner()
	if len(passed) == 0 {
		s.spinner.StopColors("fgHiYellow")
		s.spinner.StopCharacter("ran")
		s.spinner.Stop()
		return
	}

	for _, p := range passed {
		if !p {
			s.spinner.StopFail()
			return
		}
	}

	s.spinner.Stop()
}

func (s StandardTestCompleter) Passed() {
	stream.GetCoordinator().ClearActiveSpinner()
	s.spinner.Stop()
}

func (s StandardTestCompleter) Skip() {
	stream.GetCoordinator().ClearActiveSpinner()
	s.spinner.StopColors("fgHiYellow")
	s.spinner.StopCharacter("skipped")
	s.spinner.Stop()
}

func (s StandardTestCompleter) Fail() {
	stream.GetCoordinator().ClearActiveSpinner()
	s.spinner.StopFail()
}

func (s StandardTestCompleter) Error() {
	stream.GetCoordinator().ClearActiveSpinner()
	s.spinner.StopFailCharacter("error")
	s.spinner.StopFail()
}

type BaseCompleter struct {
	spinner *yacspin.Spinner
}

func padRightWithPeriods(s string, n int) string {
	if n < 1 {
		n = 1
	}
	return fmt.Sprintf("%s %s ", s, strings.Repeat(".", n))
}

func (sf *StandardFormatter) formatNodeBox(nodeName string) string {
	if sf.nodeNameWidth > 0 {
		// Pad the node name to the fixed width, accounting for the brackets
		// and internal spaces. nodeNameColor (rather than a raw ANSI escape)
		// keeps escapes out of non-terminal output: the color package
		// disables itself when stdout is not a TTY.
		paddedNodeName := fmt.Sprintf("%-*s", sf.nodeNameWidth, nodeName)
		return nodeBracketColor.Sprint("[ ") + nodeNameColor.Sprint(paddedNodeName) + nodeBracketColor.Sprint(" ]") + " "
	}
	return ""
}
