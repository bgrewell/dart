package main

import (
	"github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
)

func main() {

	cli, err := lxd.ConnectLXDUnix("", nil)
	if err != nil {
		panic(err)
	}

	// Create a request
	req := api.InstancesPost{
		Name: "test",
		Source: api.InstanceSource{
			Type:     "image",
			Alias:    "22.04",
			Server:   "https://cloud-images.ubuntu.com/releases",
			Protocol: "simplestreams",
		},
		Type: "container",
	}

	// Get LXD to create the instance
	op, err := cli.CreateInstance(req)
	if err != nil {
		panic(err)
	}

	err = op.Wait()
	if err != nil {
		panic(err)
	}

	reqState := api.InstanceStatePut{
		Action:  "start",
		Timeout: -1,
	}

	op, err = cli.UpdateInstanceState("test", reqState, "")
	if err != nil {
		panic(err)
	}

	err = op.Wait()
	if err != nil {
		panic(err)
	}
}
