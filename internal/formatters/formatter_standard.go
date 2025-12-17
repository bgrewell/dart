package formatters

import (
	"fmt"
	"github.com/bgrewell/dart/internal/results"
	"github.com/fatih/color"
	"github.com/theckman/yacspin"
	"strconv"
	"strings"
	"time"
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
	nodeNameColor      = color.New(color.FgBlack, color.BgHiYellow).Add(color.Bold)
)

func NewStandardFormatter() *StandardFormatter {
	return &StandardFormatter{
		indent:       2,
		detailIndent: 7,
	}
}

type StandardFormatter struct {
	taskColumnWidth int
	testColumnWidth int
	nodeNameWidth   int
	indent          int
	detailIndent    int
}

func (sf *StandardFormatter) PrintError(err error) {
	fmt.Printf("%s%s\n", strings.Repeat(" ", sf.detailIndent-sf.indent), valueFailColor.Sprintf(err.Error()))
}

func (sf *StandardFormatter) PrintPass(name string, details interface{}) {
	fmt.Printf("%s+%s:\n", strings.Repeat(" ", sf.detailIndent-sf.indent), headerPassColor.Sprintf(name))
	switch details.(type) {
	case string:
		lines := strings.Split(details.(string), "\n")
		for _, line := range lines {
			fmt.Printf("%s%s\n", strings.Repeat(" ", sf.detailIndent), valueColor.Sprintf(line))
		}
	case int:
		fmt.Printf("%s%s\n", strings.Repeat(" ", sf.detailIndent), valueColor.Sprintf(strconv.Itoa(details.(int))))
	}
}

func (sf *StandardFormatter) PrintFail(name string, details interface{}) {
	fmt.Printf("%s-%s:\n", strings.Repeat(" ", sf.detailIndent-sf.indent), headerFailColor.Sprintf(name))
	switch details.(type) {
	case string:
		lines := strings.Split(details.(string), "\n")
		for _, line := range lines {
			fmt.Printf("%s%s\n", strings.Repeat(" ", sf.detailIndent), valueColor.Sprintf(line))
		}
	case int:
		fmt.Printf("%s%s\n", strings.Repeat(" ", sf.detailIndent), valueColor.Sprintf(strconv.Itoa(details.(int))))
	case *results.ResultStringMatchFail:
		fmt.Printf("%s%s: %s\n", strings.Repeat(" ", sf.detailIndent), labelFailColor.Sprintf("Expected"), details.(*results.ResultStringMatchFail).Expected)
		fmt.Printf("%s%s: %s\n", strings.Repeat(" ", sf.detailIndent), labelFailColor.Sprintf("Actual"), details.(*results.ResultStringMatchFail).Actual)
	case *results.ResultIntMatchFail:
		fmt.Printf("%s%s: %d\n", strings.Repeat(" ", sf.detailIndent), labelFailColor.Sprintf("Expected"), details.(*results.ResultIntMatchFail).Expected)
		fmt.Printf("%s%s: %d\n", strings.Repeat(" ", sf.detailIndent), labelFailColor.Sprintf("Actual"), details.(*results.ResultIntMatchFail).Actual)
	}
}

func (sf *StandardFormatter) PrintEmpty() {
	fmt.Println()
}

func (sf *StandardFormatter) PrintResults(pass, fail, ran int) {

	p := 5 - (len(strconv.Itoa(pass)))
	f := 5 - (len(strconv.Itoa(fail)))
	r := 5 - (len(strconv.Itoa(ran)))

	passVal := strconv.Itoa(pass)
	failVal := strconv.Itoa(fail)
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
	ranPad := strings.Repeat("0", r)

	indent := strings.Repeat(" ", sf.indent)
	sf.PrintHeader("Results")
	fmt.Printf("%sPass: %s%s\n", indent, numberPaddingColor.Sprintf(passPad), valuePassColor.Sprintf(passVal))
	fmt.Printf("%sFail: %s%s\n", indent, numberPaddingColor.Sprintf(failPad), valueFailColor.Sprintf(failVal))
	if ran > 0 {
		fmt.Printf("%sRan:  %s%s\n", indent, numberPaddingColor.Sprintf(ranPad), valueRanColor.Sprintf(ranVal))

	}
}

func (sf *StandardFormatter) PrintHeader(header string) {
	headerPrefixColor.Printf("[+] ")
	headerColor.Printf("%s\n", header)
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
	return c
}

func (sf *StandardFormatter) StartTest(id, name, nodeName string) TestCompleter {
	spinner, _ := yacspin.New(yacspin.Config{
		Frequency:         100 * time.Millisecond,
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

	pad := strings.Repeat("0", 5-len(id))
	indent := strings.Repeat(" ", sf.indent)
	nodeBox := sf.formatNodeBox(nodeName)
	message := fmt.Sprintf("%s%s%s: %s%s", indent, numberPaddingColor.Sprintf(pad), numberColor.Sprintf(c.TestId), nodeBox, c.TestName)
	messages := []func(string){c.spinner.Message, c.spinner.StopMessage, c.spinner.StopFailMessage}
	c.spinner.Start()
	for _, m := range messages {
		m(message)
	}
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
	//s.spinner.StopMessage(fmt.Sprintf("%s%s", s.Message, "done"))
	s.spinner.Stop()
}

func (s StandardTaskCompleter) Fail() {
	//s.spinner.StopMessage(fmt.Sprintf("%s%s", s.Message, "failed"))
	s.spinner.StopFail()
}

func (s StandardTaskCompleter) Error() {
	//s.spinner.StopMessage(fmt.Sprintf("%s%s", s.Message, "error"))
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
	//s.spinner.StopMessage(fmt.Sprintf("%s %s%s", s.TestId, s.TestName, "passed"))
	s.spinner.Stop()
}

func (s StandardTestCompleter) Fail() {
	//s.spinner.StopMessage(fmt.Sprintf("%s %s%s", s.TestId, s.TestName, "failed"))
	s.spinner.StopFail()
}

func (s StandardTestCompleter) Error() {
	//s.spinner.StopMessage(fmt.Sprintf("%s %s%s", s.TestId, s.TestName, "error"))
	s.spinner.StopFailCharacter("error")
	s.spinner.StopFail()
}

type BaseCompleter struct {
	spinner *yacspin.Spinner
}

func padRightWithPeriods(s string, n int) string {
	return fmt.Sprintf("%s %s ", s, strings.Repeat(".", n))
}

func (sf *StandardFormatter) formatNodeBox(nodeName string) string {
	if nodeName != "" {
		// Pad the node name to the fixed width, accounting for the brackets
		paddedNodeName := fmt.Sprintf("%-*s", sf.nodeNameWidth, nodeName)
		return nodeNameColor.Sprintf("[%s]", paddedNodeName) + " "
	} else if sf.nodeNameWidth > 0 {
		// If no node name but we have a width set, add spacing to maintain alignment
		return strings.Repeat(" ", sf.nodeNameWidth+3) // +3 for "[ ]" and trailing space
	}
	return ""
}
