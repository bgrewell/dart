package formatters

type Formatter interface {
	SetTaskColumnWidth(width int)
	SetTestColumnWidth(width int)
	SetNodeNameWidth(width int)
	StartTask(task, nodeName, status string) TaskCompleter
	StartTest(id, name, nodeName string) TestCompleter
	PrintHeader(header string)
	PrintResults(pass, fail, skipped, ran int)
	PrintPass(name string, details interface{})
	PrintFail(name string, details interface{})
	PrintSkip(name string, reason string)
	PrintEmpty()
	PrintError(err error)
}

type TaskCompleter interface {
	Update(status string)
	Complete()
	Fail()
	Error()
}

type TestCompleter interface {
	Update(status string)
	Complete(passed []bool)
	Passed()
	Skip()
	Fail()
	Error()
}
