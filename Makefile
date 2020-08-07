default: build

build:
	go mod download
	go build -o nmc-server

nodejs:
	docker build -t nodejs.org:latest .

test:
	(cd nmc; go test -v)

clean:
	rm -f nmc-server

install:
	go install