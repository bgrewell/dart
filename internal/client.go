package internal

type ClientOptions func(cli *DartClient)

func NewDartClient(options ...ClientOptions) *DartClient {
	return &DartClient{}
}

type DartClient struct {
}
