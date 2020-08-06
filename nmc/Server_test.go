package nmc

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

func TestAddRemoveProxy(t *testing.T) {

	wd, _ := os.Getwd()
	site := path.Join(wd, "site")

	mounts := []mount.Mount{mount.Mount{Source: site, Target: "/usr/share/nginx/html", Type: mount.TypeBind}}

	for i := 0; i < 10; i++ {
		config := ContainerConfig{hostPort: "3001", containerPort: "80", containerName: "nginx-3001", imageName: imageName, mountPoint: mounts}
		proxy := CreateReverseProxy(config)

		AddProxy(proxy)
		RemoveProxy(0)
	}

	if len(proxies) != 0 {
		t.Fatalf("Proxy not deleted")
	}
}

func TestPullImage(t *testing.T) {

	ctx := context.Background()

	cli, err := client.NewEnvClient()

	if err != nil {
		panic(err)
	}

	var options types.ImagePullOptions

	buf, err := PullImage(&ctx, cli, "docker.io/library/hello-world:latest", options)

	fmt.Println(buf.String())

	if err != nil {

		t.Fatalf("Unable to pull image: " + err.Error())
	}

}
