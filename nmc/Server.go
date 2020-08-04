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

const serverPort = "3000"

const imageName = "docker.io/library/nginx:latest"
const containerName = "nginx"

var proxies = []httputil.ReverseProxy{}

var configs = []ContainerConfig{}

type ContainerConfig struct {
	hostPort      string
	containerPort string
	imageName     string
	containerName string
	mountPoint    []mount.Mount
}

func GetRoundRobinProxyIndex(configs []ContainerConfig) int {

	proxyIndex++

	if proxyIndex == len(configs) {
		proxyIndex = 0
	}

	fmt.Println("Forwarding to port: " + configs[proxyIndex].hostPort)

	return proxyIndex
}

func RoundRobinHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Forwarding request to port: " + configs[proxyIndex].hostPort)

	proxyIndex := GetRoundRobinProxyIndex(configs)

	proxies[proxyIndex].ServeHTTP(w, r)

}

func StartNginxContainers(ctx *context.Context, cli *client.Client, configs []ContainerConfig) {
	fmt.Println("Pulling latest nginx image...")

	var options types.ImagePullOptions

	reader, err := cli.ImagePull(*ctx, imageName, options)

	if err != nil {
		panic(err)
	}

	io.Copy(os.Stdout, reader)

	for i := 0; i < len(configs); i++ {

		// Portable container configuration
		var config = container.Config{Image: configs[i].imageName}

		// Non-portable container configuraton
		var portMap = make(nat.PortMap)
		port, _ := nat.NewPort("tcp", configs[i].containerPort)
		var pb nat.PortBinding
		pb.HostIP = "0.0.0.0"
		pb.HostPort = configs[i].hostPort

		portMap[port] = []nat.PortBinding{pb}

		var hostConfig = container.HostConfig{AutoRemove: true, PortBindings: portMap, Mounts: configs[i].mountPoint}

		resp, err := cli.ContainerCreate(*ctx, &config, &hostConfig, nil, configs[i].containerName)

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

func handleCtrlC(ctx *context.Context, cli *client.Client, configs []ContainerConfig) {

	c := make(chan os.Signal)

	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		duration, _ := time.ParseDuration("1m")

		for i := 0; i < len(configs); i++ {
			fmt.Println("Stopping " + configs[i].containerName + " - " + configs[i].hostPort + "...")
			cli.ContainerStop(*ctx, configs[i].containerName, &duration)
		}

		os.Exit(0)
	}()

}

func getDefaultSite() string {

	wd, _ := os.Getwd()
	site := path.Join(wd, "site")
	return site
}

func Run() {

	ctx := context.Background()
	cli, err := client.NewEnvClient()

	if err != nil {
		panic(err)
	}

	mounts := []mount.Mount{mount.Mount{Source: getDefaultSite(), Target: "/usr/share/nginx/html", Type: mount.TypeBind}}

	configs = append(configs, ContainerConfig{hostPort: "3001", containerPort: "80", containerName: "nginx-3001", imageName: imageName, mountPoint: mounts})
	configs = append(configs, ContainerConfig{hostPort: "3002", containerPort: "80", containerName: "nginx-3002", imageName: imageName, mountPoint: mounts})

	StartNginxContainers(&ctx, cli, configs)

	handleCtrlC(&ctx, cli, configs)

	for i := 0; i < len(configs); i++ {
		origin, _ := url.Parse("http://localhost:" + configs[i].hostPort + "/")

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

	log.Fatal(http.ListenAndServe(":"+serverPort, nil))

}
