package formatters

type Formatter interface {
	SetTaskColumnWidth(width int)
	SetTestColumnWidth(width int)
	StartTask(task, status string) TaskCompleter
	StartTest(id, name string) TestCompleter
	PrintHeader(header string)
	PrintResults(pass, fail int)
	PrintPass(name string, details interface{})
	PrintFail(name string, details interface{})
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
	Fail()
	Error()
}
