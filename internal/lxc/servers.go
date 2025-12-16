package lxc

import (
	"fmt"
)

// Common image servers for LXD/LXC
var imageServers = map[string]struct {
	url      string
	protocol string
}{
	"ubuntu": {
		url:      "https://cloud-images.ubuntu.com/releases",
		protocol: "simplestreams",
	},
	"images": {
		url:      "https://images.linuxcontainers.org",
		protocol: "simplestreams",
	},
	"lxc": {
		url:      "https://images.linuxcontainers.org",
		protocol: "simplestreams",
	},
}

// GetUrlAndProtocol returns the URL and protocol for a given image server alias
// Common aliases are "ubuntu" for Ubuntu Cloud Images and "images" or "lxc" for Linux Containers images
func GetUrlAndProtocol(alias string) (url, protocol string, err error) {
	server, ok := imageServers[alias]
	if !ok {
		return "", "", fmt.Errorf("unknown image server alias: %s", alias)
	}
	return server.url, server.protocol, nil
}
