build:
	go mod download
	go build -o nmc-server

default: build

test:
	(cd nmc; go test)

clean:
	rm -f nmc-server

install:
	go install