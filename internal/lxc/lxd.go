package lxc

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bgrewell/go-execute/v2"
)

func GetUrlAndProtocol(name string) (url string, protocol string, err error) {
	stdout, stderr, err := execute.ExecuteSeparate("lxc remote list -f json")
	if stderr != "" {
		return "", "", fmt.Errorf("Error getting remote list: %s", stderr)
	} else if err != nil {
		return "", "", err
	}

	// Parse stdout into a map where the key is the remote name and the value is a Remote struct.
	var remoteMap map[string]Remote
	err = json.Unmarshal([]byte(stdout), &remoteMap)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}

	// Look up the remote by name
	for key, remote := range remoteMap {
		if key == name {
			return remote.Addr, remote.Protocol, nil
		}
	}

	return "", "", errors.New("failed to find the specified remote")
}
