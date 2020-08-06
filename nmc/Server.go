package nmc

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/theckman/yacspin"
)

// internal counter used to track which port to forward requests to
var proxyIndex int

//const serverPort = "3000"

const imageName = "docker.io/library/nginx"
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

// AddProxy adds a reverse proxy to proxies
func AddProxy(proxy httputil.ReverseProxy) {
	proxies = append(proxies, proxy)
}

// RemoveProxy removes a reverse proxy from proxies
func RemoveProxy(index int) {
	// Set the current proxy to the last proxy in the list
	proxies[index] = proxies[len(proxies)-1]

	// Return a slice of proxies, which exludes the last one (effectively orphaning it)
	proxies = proxies[:len(proxies)-1]
}

// DoRoundRobin proxies each request to the next proxy.
func DoRoundRobin(ctx *context.Context, cli *client.Client, proxies []httputil.ReverseProxy) {

	var proxyIndex = 0

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		proxyIndex++

		if proxyIndex == len(proxies) {
			proxyIndex = 0
		}

		proxy := proxies[proxyIndex]

		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
			message := "Handling error for " + configs[proxyIndex].hostPort + "\n"

			log.Println(message)

			// Let the client know that the request failed
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)

			go func() {
				RemoveProxy(proxyIndex)

				log.Println("Stopping container")

				err := StopContainer(ctx, cli, configs[proxyIndex].body)

				if err != nil {
					log.Println("Container could not be stopped")
				}

				log.Println("Creating container")

				body, err := CreateContainer(ctx, cli, configs[proxyIndex])

				if err != nil {
					log.Println("Container could not be created")
					panic(err)
				}

				// Update container ID
				configs[proxyIndex].body = body

				log.Println("Starting container")

				err = StartContainer(ctx, cli, body)

				if err != nil {
					log.Println("Container could not be started")
					panic(err)
				}

				HandleCtrlC(ctx, cli, configs[proxyIndex])

				proxy = CreateReverseProxy(configs[proxyIndex])

				AddProxy(proxy)

				log.Println("Proxy available")
			}()
			log.Println("Re-creating proxy...")
			return

		}

		log.Println("Forwarding request to port: " + configs[proxyIndex].hostPort)

		proxy.ServeHTTP(w, r)
	})

}

// PullImage pulls an image
func PullImage(ctx *context.Context, cli *client.Client, imageName string, options types.ImagePullOptions) (*bytes.Buffer, error) {

	reader, err := cli.ImagePull(*ctx, imageName, options)

	if err != nil {
		log.Fatal("Unable to pull image " + imageName)
		//panic(err)
	}

	defer reader.Close()

	// Create a pointer to a buffer that will hold the output
	buf := new(bytes.Buffer)

	buf.ReadFrom(reader)

	return buf, err
}

// CreateContainer creates a container from ContainerConfig
func CreateContainer(ctx *context.Context, cli *client.Client, config ContainerConfig) (container.ContainerCreateCreatedBody, error) {

	// Portable container configuration
	var containerConfig = &container.Config{
		Image:        config.imageName,
		Tty:          true,
		AttachStdout: true,
		AttachStderr: true,
		ExposedPorts: nat.PortSet{
			nat.Port("80/tcp"): {},
		},
	}

	hostConfig := &container.HostConfig{
		Binds: []string{
			"/var/run/docker.sock:/var/run/docker.sock",
		},
		PortBindings: nat.PortMap{
			nat.Port("80/tcp"): []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: config.hostPort}},
		},
	}

	body, err := cli.ContainerCreate(*ctx, containerConfig, hostConfig, nil, config.containerName)

	config.body = body

	return body, err
}

// StartContainer starts a container
func StartContainer(ctx *context.Context, cli *client.Client, body container.ContainerCreateCreatedBody) error {

	return cli.ContainerStart(*ctx, body.ID, types.ContainerStartOptions{})
}

// StopContainer stops a container
func StopContainer(ctx *context.Context, cli *client.Client, body container.ContainerCreateCreatedBody) error {

	duration, _ := time.ParseDuration("1m")

	return cli.ContainerStop(*ctx, body.ID, &duration)
}

// HandleCtrlC stops a container if it receives a signal.
func HandleCtrlC(ctx *context.Context, cli *client.Client, config ContainerConfig) {

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

// CreateReverseProxy creates a reverse proxy
func CreateReverseProxy(config ContainerConfig) httputil.ReverseProxy {

	origin, _ := url.Parse("http://localhost:" + config.hostPort + "/")

	director := func(req *http.Request) {
		req.Header.Add("X-Forwarded-Host", req.Host)
		req.Header.Add("X-Origin-Host", origin.Host)
		req.URL.Scheme = "http"
		req.URL.Host = origin.Host
	}

	return httputil.ReverseProxy{Director: director}
}

// Run runs the proxies and main HTTP server. The site is the path where files will be served.
func Run(site string, count int, serverPort int, port int, imageVersion string) {

	//log.Println("Starting nmc...")

	cfg := yacspin.Config{
		Frequency:       100 * time.Millisecond,
		CharSet:         yacspin.CharSets[25],
		Suffix:          " Starting Nginx Mini-cluster... ",
		SuffixAutoColon: true,
		StopCharacter:   "✓",
		StopColors:      []string{"fgGreen"},
	}

	spinner, _ := yacspin.New(cfg)

	spinner.Start()

	ctx := context.Background()

	cli, err := client.NewEnvClient()

	if err != nil {
		panic(err)
	}

	var options types.ImagePullOptions

	buf, err := PullImage(&ctx, cli, imageName+":"+imageVersion, options)

	if err != nil {
		log.Println(buf.String())
		panic(err)
	}

	mounts := []mount.Mount{mount.Mount{Source: site, Target: "/usr/share/nginx/html", Type: mount.TypeBind}}

	var policy container.RestartPolicy
	policy.IsAlways()

	// Start as many proxies as the user specified, default is specified in root.go
	for i := 0; i < count; i++ {
		configs = append(configs, ContainerConfig{hostPort: strconv.Itoa(port + i), containerPort: "80", containerName: "nginx-" + strconv.Itoa(port+i), imageName: imageName, mountPoint: mounts})
	}

	// configs = append(configs, ContainerConfig{hostPort: "3001", containerPort: "80", containerName: "nginx-3001", imageName: imageName, mountPoint: mounts})
	// configs = append(configs, ContainerConfig{hostPort: "3002", containerPort: "80", containerName: "nginx-3002", imageName: imageName, mountPoint: mounts})

	for _, config := range configs {
		body, err := CreateContainer(&ctx, cli, config)

		if err != nil {
			panic(err)
		}

		err = StartContainer(&ctx, cli, body)

		if err != nil {
			panic(err)
		}

		HandleCtrlC(&ctx, cli, config)

		proxy := CreateReverseProxy(config)

		AddProxy(proxy)

	}

	// Guard against creating proxies if we don't allow any
	if count > 0 {
		DoRoundRobin(&ctx, cli, proxies)
	}

	spinner.Stop()

	err = http.ListenAndServe(":"+strconv.Itoa(serverPort), nil)

	if err != nil {
		log.Fatal(err)
	}

}
