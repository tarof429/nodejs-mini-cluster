package nmc

import (
	"testing"
)

func TestAddRemoveProxy(t *testing.T) {

	config := ContainerConfig{hostPort: "3001", containerPort: "80", containerName: "nginx-3001", imageName: imageName}
	proxy := CreateReverseProxy(config)

	AddProxy(proxy)
	RemoveProxy(0)

}
