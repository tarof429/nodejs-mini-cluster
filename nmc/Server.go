package nmc

import (
	"fmt"
	"net/http"
)

var forwardPortCounter int
var forwardPorts = []string{"3001", "3002"}

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
	}
}

func Run() {

	http.HandleFunc("/", RoundRobinHandler)
	fmt.Println("Server starting...")

	http.ListenAndServe(":3000", nil)
}

// func main() {
// 	http.HandleFunc("/", RoundRobinHandler)
// 	fmt.Println("Server starting...")

// 	http.ListenAndServe(":3000", nil)
// }
