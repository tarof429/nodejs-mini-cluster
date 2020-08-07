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
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/theckman/yacspin"
)

const (
	hostPort         = "3000"
	imageName        = "nodejs.org"
	containerName    = "nodejs.org"
	containerVersion = "latest"
	nginxURL         = "http://localhost:8080"
)

// internal counter used to track which port to forward requests to
var (
	proxies     = []httputil.ReverseProxy{} // List of reverse proxies
	proxyIndex  int                         // The current proxy index
	configs     = []ContainerConfig{}       // Docker container config
	serverState chan string                 // Global channel for server state
)

// ContainerConfig is the configuration of the docker container
type ContainerConfig struct {
	hostPort         string
	containerPort    string
	imageName        string
	containerName    string
	containerVersion string
	body             container.ContainerCreateCreatedBody
}

// AddProxy adds a reverse proxy to proxies
func AddProxy(proxy httputil.ReverseProxy) {
	proxies = append(proxies, proxy)
}

// RemoveProxy removes a reverse proxy from proxies
func RemoveProxy(index int) {
	if index > 0 && index < len(proxies) {
		// Set the current proxy to the last proxy in the list
		proxies[index] = proxies[len(proxies)-1]

		// Return a slice of proxies, which exludes the last one (effectively orphaning it)
		proxies = proxies[:len(proxies)-1]
	}
}

func ResetProxy(ctx *context.Context, cli *client.Client, proxyIndex int) {

	log.Println("Removing proxy")
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
		return
	}

	// Update container ID
	configs[proxyIndex].body = body

	log.Println("Starting container")

	err = StartContainer(ctx, cli, body)

	if err != nil {
		log.Println("Container could not be started")
		return
	}

	HandleCtrlC(ctx, cli, configs[proxyIndex])

	proxy := CreateReverseProxy(configs[proxyIndex])

	AddProxy(proxy)

	log.Println("Proxy available")
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

			select {
			case currentState := <-serverState:
				if currentState != "Ready" {
					log.Println("Server is not ready. State is " + currentState + ".")
					return
				}
			default:
				message := "Handling error for " + configs[proxyIndex].hostPort + "\n"
				log.Println(message)
			}

			// Let the client know that the request failed
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)

			go ResetProxy(ctx, cli, proxyIndex)
			log.Println("Re-creating proxy...")

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
	}

	defer reader.Close()

	// Create a pointer to a buffer that will hold the output
	buf := new(bytes.Buffer)

	buf.ReadFrom(reader)

	return buf, err
}

// CreateContainer creates a container from ContainerConfig
func CreateContainer(ctx *context.Context, cli *client.Client, config ContainerConfig) (container.ContainerCreateCreatedBody, error) {

	healthConfig := container.HealthConfig{
		Interval: time.Duration(time.Duration.Minutes(1)),
		Retries:  3,
		Test:     []string{"curl", nginxURL},
		Timeout:  time.Duration(time.Duration.Seconds(10)),
	}

	// Portable container configuration
	containerConfig := &container.Config{
		Image:        config.imageName,
		Tty:          true,
		AttachStdout: true,
		AttachStderr: true,
		ExposedPorts: nat.PortSet{
			nat.Port("8080/tcp"): {},
		},
		Healthcheck: &healthConfig,
	}

	hostConfig := &container.HostConfig{
		// Binds: []string{
		// 	"/var/run/docker.sock:/var/run/docker.sock",
		// },
		PortBindings: nat.PortMap{
			nat.Port("8080/tcp"): []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: config.hostPort}},
		},
		AutoRemove: true,
	}

	body, err := cli.ContainerCreate(*ctx, containerConfig, hostConfig, nil, config.containerName)

	config.body = body

	return body, err
}

// StartContainer starts a container
func StartContainer(ctx *context.Context, cli *client.Client, body container.ContainerCreateCreatedBody) error {

	return cli.ContainerStart(*ctx, body.ID, types.ContainerStartOptions{})
}

// HealthcheckURL checks if the URL is up
func HealthcheckURL(url string, retries int) error {

	var err error

	time.Sleep(3 * time.Second)

	for retry := 0; retry < retries; retry++ {

		_, err = http.Get(url)

		if err != nil {
			time.Sleep(3 * time.Second)
		} else {
			return nil
		}
	}
	return err
}

// StopContainer stops a container
func StopContainer(ctx *context.Context, cli *client.Client, body container.ContainerCreateCreatedBody) error {

	duration, _ := time.ParseDuration("1m")

	// if body.ID == "" {
	// 	serverState <- "Invalid"
	// }
	//log.Println("Stopping container with ID " + body.ID)
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

// Run runs the proxies and main HTTP server.
func Run() {

	serverState = make(chan string)

	cfg := yacspin.Config{
		Frequency:       100 * time.Millisecond,
		CharSet:         yacspin.CharSets[53],
		Suffix:          " Starting cluster... ",
		SuffixAutoColon: true,
		StopCharacter:   "✓",
		StopColors:      []string{"fgGreen"},
	}

	spinner, _ := yacspin.New(cfg)

	spinner.Start()

	go func() {
		serverState <- "Starting"
	}()

	ctx := context.Background()

	cli, err := client.NewEnvClient()

	if err != nil {
		panic(err)
	}

	// Start as many proxies as the user specified, default is specified in root.go
	// for i := 0; i < count; i++ {
	// 	configs = append(configs, ContainerConfig{hostPort: strconv.Itoa(port + i), containerPort: "80", containerName: "nginx-" + strconv.Itoa(port+i), imageName: imageName})
	// }

	configs = append(configs, ContainerConfig{hostPort: "3001", containerPort: "8080", containerName: "nginx-3001", imageName: imageName})
	configs = append(configs, ContainerConfig{hostPort: "3002", containerPort: "8080", containerName: "nginx-3002", imageName: imageName})

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

	DoRoundRobin(&ctx, cli, proxies)

	// Waitgroup
	wg := new(sync.WaitGroup)
	wg.Add(1)

	go func() {
		log.Fatal(http.ListenAndServe(":"+hostPort, nil))
		wg.Done() // goroutine for http server is done
	}()

	err = HealthcheckURL("http://localhost:"+hostPort, 30)

	if err != nil {
		panic(err)
	}

	spinner.Stop()

	// Consume from the channel and set a new state saying that we're ready
	<-serverState
	serverState <- "Ready"

	// Wait until waitgroup is done
	wg.Wait()
}
