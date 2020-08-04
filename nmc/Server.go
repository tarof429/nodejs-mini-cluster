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
	body          container.ContainerCreateCreatedBody
	mountPoint    []mount.Mount
}

func GetRoundRobinProxyIndex() int {

	proxyIndex++

	if proxyIndex == len(configs) {
		proxyIndex = 0
	}

	return proxyIndex
}

func DoRoundRobin(ctx *context.Context, cli *client.Client, proxies []httputil.ReverseProxy) {

	var proxyIndex = 0

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		proxyIndex++

		if proxyIndex == len(configs) {
			proxyIndex = 0
		}

		proxy := proxies[proxyIndex]

		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
			message := "Handling error for " + configs[proxyIndex].hostPort + "\n"
			//w.Write([]byte(message))
			log.Println(message)

			go func() {
				err := StopContainer(ctx, cli, configs[proxyIndex].body)

				if err != nil {
					fmt.Println(err)
				}

				body, err := CreateContainer(ctx, cli, configs[proxyIndex])

				if err != nil {
					panic(err)
				}

				// Update container ID
				configs[proxyIndex].body = body

				err = StartContainer(ctx, cli, body)

				if err != nil {
					panic(err)
				}

				handleCtrlC(ctx, cli, configs[proxyIndex])

				proxy = CreateReverseProxy(configs[proxyIndex])

				proxies[proxyIndex] = proxy
			}()

		}

		log.Println("Forwarding request to port: " + configs[proxyIndex].hostPort)

		proxy.ServeHTTP(w, r)

	})

}

func RoundRobinHandler(w http.ResponseWriter, r *http.Request) {

	proxyIndex := GetRoundRobinProxyIndex()

	fmt.Println("Forwarding request to port: " + configs[proxyIndex].hostPort)

	proxy := proxies[proxyIndex]

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		message := "Handling error for " + configs[proxyIndex].hostPort + "\n"
		w.Write([]byte(message))

	}
	proxy.ServeHTTP(w, r)
}

func CreateContainer(ctx *context.Context, cli *client.Client, config ContainerConfig) (container.ContainerCreateCreatedBody, error) {

	var options types.ImagePullOptions

	reader, err := cli.ImagePull(*ctx, imageName, options)

	if err != nil {
		panic(err)
	}

	io.Copy(os.Stdout, reader)

	defer reader.Close()

	// Portable container configuration
	var containerConfig = container.Config{Image: config.imageName}

	// Non-portable container configuraton
	var portMap = make(nat.PortMap)
	port, _ := nat.NewPort("tcp", config.containerPort)
	var pb nat.PortBinding
	pb.HostIP = "0.0.0.0"
	pb.HostPort = config.hostPort

	portMap[port] = []nat.PortBinding{pb}

	var hostConfig = container.HostConfig{AutoRemove: true, PortBindings: portMap, Mounts: config.mountPoint}

	body, err := cli.ContainerCreate(*ctx, &containerConfig, &hostConfig, nil, config.containerName)

	config.body = body

	return body, err
}

func StartContainer(ctx *context.Context, cli *client.Client, body container.ContainerCreateCreatedBody) error {

	return cli.ContainerStart(*ctx, body.ID, types.ContainerStartOptions{})
}

func StopContainer(ctx *context.Context, cli *client.Client, body container.ContainerCreateCreatedBody) error {

	duration, _ := time.ParseDuration("1m")

	return cli.ContainerStop(*ctx, body.ID, &duration)
}

// func StartContainers(ctx *context.Context, cli *client.Client, configs []ContainerConfig) {

// 	var options types.ImagePullOptions

// 	reader, err := cli.ImagePull(*ctx, imageName, options)

// 	if err != nil {
// 		panic(err)
// 	}

// 	io.Copy(os.Stdout, reader)

// 	for i := 0; i < len(configs); i++ {

// 		// Portable container configuration
// 		var config = container.Config{Image: configs[i].imageName}

// 		// Non-portable container configuraton
// 		var portMap = make(nat.PortMap)
// 		port, _ := nat.NewPort("tcp", configs[i].containerPort)
// 		var pb nat.PortBinding
// 		pb.HostIP = "0.0.0.0"
// 		pb.HostPort = configs[i].hostPort

// 		portMap[port] = []nat.PortBinding{pb}

// 		var hostConfig = container.HostConfig{AutoRemove: true, PortBindings: portMap, Mounts: configs[i].mountPoint}

// 		resp, err := cli.ContainerCreate(*ctx, &config, &hostConfig, nil, configs[i].containerName)

// 		if err != nil {
// 			panic(err)
// 		}

// 		if err := cli.ContainerStart(*ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
// 			panic(err)
// 		}

// 		fmt.Println(resp.ID)
// 	}

// 	defer reader.Close()

// }

// func handleCtrlCX(ctx *context.Context, cli *client.Client, configs []ContainerConfig) {

// 	c := make(chan os.Signal)

// 	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

// 	go func() {
// 		<-c
// 		duration, _ := time.ParseDuration("1m")

// 		for i := 0; i < len(configs); i++ {
// 			fmt.Println("Stopping " + configs[i].containerName + " - " + configs[i].hostPort + "...")
// 			cli.ContainerStop(*ctx, configs[i].containerName, &duration)
// 		}

// 		os.Exit(0)
// 	}()

// }

func handleCtrlC(ctx *context.Context, cli *client.Client, config ContainerConfig) {

	c := make(chan os.Signal)

	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		duration, _ := time.ParseDuration("1m")

		fmt.Println("Stopping " + config.containerName + " - " + config.hostPort + "...")
		cli.ContainerStop(*ctx, config.containerName, &duration)

		os.Exit(0)
	}()

}

func getDefaultSite() string {

	wd, _ := os.Getwd()
	site := path.Join(wd, "site")
	return site
}

func CreateReverseProxy(config ContainerConfig) httputil.ReverseProxy {

	origin, _ := url.Parse("http://localhost:" + config.hostPort + "/")

	//fmt.Println("origin: " + origin.Host)

	director := func(req *http.Request) {
		req.Header.Add("X-Forwarded-Host", req.Host)
		req.Header.Add("X-Origin-Host", origin.Host)
		req.URL.Scheme = "http"
		req.URL.Host = origin.Host
	}

	return httputil.ReverseProxy{Director: director}
}

func Run() {

	ctx := context.Background()
	cli, err := client.NewEnvClient()

	if err != nil {
		panic(err)
	}

	mounts := []mount.Mount{mount.Mount{Source: getDefaultSite(), Target: "/usr/share/nginx/html", Type: mount.TypeBind}}

	var policy container.RestartPolicy
	policy.IsAlways()

	configs = append(configs, ContainerConfig{hostPort: "3001", containerPort: "80", containerName: "nginx-3001", imageName: imageName, mountPoint: mounts})
	configs = append(configs, ContainerConfig{hostPort: "3002", containerPort: "80", containerName: "nginx-3002", imageName: imageName, mountPoint: mounts})

	for _, config := range configs {
		body, err := CreateContainer(&ctx, cli, config)

		if err != nil {
			panic(err)
		}

		err = StartContainer(&ctx, cli, body)

		if err != nil {
			panic(err)
		}

		handleCtrlC(&ctx, cli, config)

		proxy := CreateReverseProxy(config)

		proxies = append(proxies, proxy)
	}

	//http.HandleFunc("/", RoundRobinHandler)
	DoRoundRobin(&ctx, cli, proxies)

	log.Fatal(http.ListenAndServe(":"+serverPort, nil))

}
