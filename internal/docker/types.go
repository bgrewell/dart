package docker

// ContainerFilter holds criteria for filtering containers
type ContainerFilter struct {
	Name   string
	Status string
	Image  string
	Label  string
}

type Network struct {
	Name    string `json:"name" yaml:"name"`
	Subnet  string `json:"subnet" yaml:"subnet"`
	Gateway string `json:"gateway" yaml:"gateway"`
}

type Image struct {
	Name       string `json:"name" yaml:"name"`
	Tag        string `json:"tag" yaml:"tag"`
	Dockerfile string `json:"dockerfile" yaml:"dockerfile"`
}
