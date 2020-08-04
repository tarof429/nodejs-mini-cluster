package nmc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// internal counter used to track which port to forward requests to
var forwardPortCounter int

// List of forwarding ports
var forwardPorts = []string{"3001", "3002"}

//var forwardPorts = []string{"3001"}

const imageName = "docker.io/library/nginx:latest"
const containerName = "nginx"

func GetRoundRobinForwardPort() string {

	forwardPortCounter++

	if forwardPortCounter == len(forwardPorts) {
		forwardPortCounter = 0
	}

	fmt.Println("Forwarding to port: " + forwardPorts[forwardPortCounter])

	return forwardPorts[forwardPortCounter]
}

func RoundRobinHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Called RoundRobinHandler")

	forwardPort := GetRoundRobinForwardPort()

	switch method := r.Method; method {

	case "GET":

		fmt.Println("Forwarding request to port: " + forwardPort)

		http.Redirect(w, r, "http://localhost:"+forwardPort, http.StatusOK)

	}
}

func StartNginx() {
	fmt.Println("Pulling latest nginx image...")

	ctx := context.Background()
	cli, err := client.NewEnvClient()

	if err != nil {
		panic(err)
	}

	var options types.ImagePullOptions

	reader, err := cli.ImagePull(ctx, imageName, options)

	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, reader)

	for i := 0; i < len(forwardPorts); i++ {

		// Portable container configuration
		var config = container.Config{Image: imageName}

		// Non-portable container configuraton
		var portMap = make(nat.PortMap)
		port, _ := nat.NewPort("tcp", "80")
		var pb nat.PortBinding
		pb.HostIP = "0.0.0.0"
		pb.HostPort = forwardPorts[i]

		portMap[port] = []nat.PortBinding{pb}

		mounts := []mount.Mount{}
		m := mount.Mount{Source: "/home/taro/Code/Go/nginx-mini-cluster/site", Target: "/usr/share/nginx/html", Type: mount.TypeBind}
		mounts = append(mounts, m)

		var hostConfig = container.HostConfig{AutoRemove: true, PortBindings: portMap, Mounts: mounts}

		resp, err := cli.ContainerCreate(ctx, &config, &hostConfig, nil, containerName+"-"+forwardPorts[i])

		if err != nil {
			panic(err)
		}

		if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
			panic(err)
		}

		fmt.Println(resp.ID)
	}

	defer reader.Close()
}

func Run() {

	StartNginx()

	http.HandleFunc("/", RoundRobinHandler)
	fmt.Println("Server starting...")

	http.ListenAndServe(":3000", nil)
}

// func main() {
// 	http.HandleFunc("/", RoundRobinHandler)
// 	fmt.Println("Server starting...")

// 	http.ListenAndServe(":3000", nil)
// }
