package formatters

import (
	"fmt"
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
	numberColor        = color.New(color.FgHiCyan)
	numberPaddingColor = color.New(color.FgHiBlack)
	passNumberColor    = color.New(color.FgHiGreen)
	failNumberColor    = color.New(color.FgHiRed)
)

func NewStandardFormatter() *StandardFormatter {
	return &StandardFormatter{
		indent: 2,
	}
}

type StandardFormatter struct {
	taskColumnWidth int
	testColumnWidth int
	indent          int
}

func (sf *StandardFormatter) PrintEmpty() {
	fmt.Println()
}

func (sf *StandardFormatter) PrintResults(pass, fail int) {

	p := 5 - (len(strconv.Itoa(pass)))
	f := 5 - (len(strconv.Itoa(fail)))

	passVal := passNumberColor.Sprintf(strconv.Itoa(pass))
	failVal := failNumberColor.Sprintf(strconv.Itoa(fail))

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

	indent := strings.Repeat(" ", sf.indent)
	sf.PrintHeader("Results")
	fmt.Printf("%sPass: %s%s\n", indent, numberPaddingColor.Sprintf(passPad), passNumberColor.Sprintf(passVal))
	fmt.Printf("%sFail: %s%s\n", indent, numberPaddingColor.Sprintf(failPad), failNumberColor.Sprintf(failVal))
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

func (sf *StandardFormatter) StartTask(task, status string) TaskCompleter {

	spinner, _ := yacspin.New(yacspin.Config{
		Frequency:         100 * time.Millisecond,
		ShowCursor:        false,
		SpinnerAtEnd:      true,
		CharSet:           yacspin.CharSets[14],
		Colors:            []string{"fgHiGreen"},
		StopColors:        []string{"fgHiGreen"},
		StopFailColors:    []string{"fgHiRed"},
		StopFailCharacter: "✗",
		StopCharacter:     "✓",
	})
	c := &StandardTaskCompleter{
		BaseCompleter: BaseCompleter{
			spinner: spinner,
		},
		Message: padRightWithPeriods(task, sf.taskColumnWidth-len(task)+3),
	}

	indent := strings.Repeat(" ", sf.indent)
	message := fmt.Sprintf("%s%s", indent, c.Message)
	messages := []func(string){c.spinner.Message, c.spinner.StopMessage, c.spinner.StopFailMessage}
	c.spinner.Start()
	for _, m := range messages {
		m(message)
	}
	return c
}

func (sf *StandardFormatter) StartTest(id, name string) TestCompleter {
	spinner, _ := yacspin.New(yacspin.Config{
		Frequency:         100 * time.Millisecond,
		ShowCursor:        false,
		SpinnerAtEnd:      true,
		CharSet:           yacspin.CharSets[14],
		Colors:            []string{"fgHiGreen"},
		StopColors:        []string{"fgHiGreen"},
		StopFailColors:    []string{"fgHiRed"},
		StopFailCharacter: "✗",
		StopCharacter:     "✓",
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
	message := fmt.Sprintf("%s%s%s: %s", indent, numberPaddingColor.Sprintf(pad), numberColor.Sprintf(c.TestId), c.TestName)
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
	s.spinner.StopColors()
	chars := []string{}
	for _, p := range passed {
		if p {
			chars = append(chars, passNumberColor.Sprintf("✓"))
		} else {
			chars = append(chars, failNumberColor.Sprintf("✗"))
		}
	}
	s.spinner.StopCharacter(strings.Join(chars, " "))
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
	s.spinner.StopFail()
}

type BaseCompleter struct {
	spinner *yacspin.Spinner
}

func padRightWithPeriods(s string, n int) string {
	return fmt.Sprintf("%s %s ", s, strings.Repeat(".", n))
}
