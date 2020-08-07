# nodejs mini-cluster

## Introduction

nodejs mini-cluster uses reverse proxies to run a nodejs cluster running in docker containers. The docker image used with this project is the nodejs website.

## Details

When nmc-server starts, it will create and run docker containers which run the nodejs website. The server routes trafic to each container in round-robin fashion. If a container becomes inaccessible it will be restarted automatically. A monitor goroutine periodically checks whether each proxy is available and redeploys the container if inacccesible.

By default, the server port (the port clients should connect to) is set to 3000. If you *curl http://localhost:3000*, each request will be routed to each container in round-robin fashion.

```bash
                                                       ----------
                                                3001   | nodejs  |
                                             --------------------
                                             |         | docker |
                           --------------    |         ----------
 http://localhost:3000 ->  | nmc-server | ---|         ----------
                           --------------    |         | nodejs  |
                                             ---------------------
                                                3002   | docker |  
                                                       ----------
```

## Building

If this is the first time to build, you must build the docker container.

```bash
$ make nodejs
```

Afterwards just run make

```bash
$ make
```

## Running

```bash
$ ./nmc-server
✓ Starting cluster...
```

Confirm that the containers are running.

```bash
$ docker ps|grep nodejs
1fabae0e9ddc        nodejs.org                "/usr/local/bin/npm …"   16 seconds ago      Up 15 seconds           0.0.0.0:3002->8080/tcp                                         nginx-3002
e7599c8bc087        nodejs.org                "/usr/local/bin/npm …"   17 seconds ago      Up 15 seconds           0.0.0.0:3001->8080/tcp                                         nginx-3001
```

Point your browser to:

```bash
$ curl http://localhost:3000
```

Back in the first terminal, you should see the following output, illustrating that round-robin clustering is working.

```bash
2020/08/05 22:16:19 Forwarding request to port: 3002
2020/08/05 22:16:30 Forwarding request to port: 3001
```

There are several options for running the server:

```bash
$ ./nmc-server -h
A mini cluster using docker and nodejs

Usage:
  nodejs-mini-cluster [flags]

Flags:
  -h, --help      help for nodejs-mini-cluster
  -v, --version   version for nodejs-mini-cluster
```