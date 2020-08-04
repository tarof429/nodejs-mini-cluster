package nmc

import (
	"testing"

	"github.com/docker/docker/api/types/mount"
)

func TestAddRemoveProxy(t *testing.T) {

	mounts := []mount.Mount{mount.Mount{Source: getDefaultSite(), Target: "/usr/share/nginx/html", Type: mount.TypeBind}}

	for i := 0; i < 10; i++ {
		config := ContainerConfig{hostPort: "3001", containerPort: "80", containerName: "nginx-3001", imageName: imageName, mountPoint: mounts}
		proxy := CreateReverseProxy(config)

		AddProxy(proxy)
	}

	RemoveProxy(0)

	if len(proxies) != 9 {
		t.Fatalf("Proxy not deleted")
	}
}
