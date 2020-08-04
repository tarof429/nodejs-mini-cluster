package nmc

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// internal counter used to track which port to forward requests to
var proxyIndex int

// List of forwarding ports
var forwardPorts = []string{"3001", "3002"}

//var forwardPorts = []string{"3001"}

const imageName = "docker.io/library/nginx:latest"
const containerName = "nginx"

var proxies = []httputil.ReverseProxy{}

func GetRoundRobinProxyIndex() int {

	proxyIndex++

	if proxyIndex == len(forwardPorts) {
		proxyIndex = 0
	}

	fmt.Println("Forwarding to port: " + forwardPorts[proxyIndex])

	return proxyIndex
}

func RoundRobinHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Forwarding request to port: " + forwardPorts[proxyIndex])

	proxyIndex := GetRoundRobinProxyIndex()

	proxies[proxyIndex].ServeHTTP(w, r)

}

func StartNginxContainers(ctx *context.Context, cli *client.Client) {
	fmt.Println("Pulling latest nginx image...")

	var options types.ImagePullOptions

	reader, err := cli.ImagePull(*ctx, imageName, options)

	if err != nil {
		panic(err)
	}

	io.Copy(os.Stdout, reader)

	wd, _ := os.Getwd()
	site := path.Join(wd, "site")

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

		m := mount.Mount{Source: site, Target: "/usr/share/nginx/html", Type: mount.TypeBind}
		mounts = append(mounts, m)

		var hostConfig = container.HostConfig{AutoRemove: true, PortBindings: portMap, Mounts: mounts}

		resp, err := cli.ContainerCreate(*ctx, &config, &hostConfig, nil, containerName+"-"+forwardPorts[i])

		if err != nil {
			panic(err)
		}

		if err := cli.ContainerStart(*ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
			panic(err)
		}

		fmt.Println(resp.ID)
	}

	defer reader.Close()

}

func handleCtrlC(ctx *context.Context, cli *client.Client) {

	c := make(chan os.Signal)

	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		duration, _ := time.ParseDuration("1m")

		for i := 0; i < len(forwardPorts); i++ {
			fmt.Println("Stopping " + containerName + " - " + forwardPorts[i] + "...")
			cli.ContainerStop(*ctx, containerName+"-"+forwardPorts[i], &duration)
		}

		os.Exit(0)
	}()

}

func Run() {

	ctx := context.Background()
	cli, err := client.NewEnvClient()

	if err != nil {
		panic(err)
	}

	StartNginxContainers(&ctx, cli)

	handleCtrlC(&ctx, cli)

	for i := 0; i < len(forwardPorts); i++ {
		origin, _ := url.Parse("http://localhost:" + forwardPorts[i] + "/")

		director := func(req *http.Request) {
			req.Header.Add("X-Forwarded-Host", req.Host)
			req.Header.Add("X-Origin-Host", origin.Host)
			req.URL.Scheme = "http"
			req.URL.Host = origin.Host
		}

		proxy := httputil.ReverseProxy{Director: director}

		proxies = append(proxies, proxy)
	}

	http.HandleFunc("/", RoundRobinHandler)

	log.Fatal(http.ListenAndServe(":3000", nil))

}
