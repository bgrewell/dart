package docker

// ContainerFilter holds criteria for filtering containers
type ContainerFilter struct {
	Name   string
	Status string
	Image  string
	Label  string
}
