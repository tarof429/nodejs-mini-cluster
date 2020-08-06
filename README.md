# NGINX Mini-Cluster

## Introduction

NGINX Mini-Cluster runs an nginx cluster. It lets you serve web sites with some level of high-availability.

## Details

When nc-server starts, it will create and run nginx docker containers when serve the site directory. The server routes trafic to each container in round-robin fashion. If a container becomes inaccessible it will be restarted automatically. 

By default, the server port (the port clients should connect to) is set to 3000. If you *curl http://localhost:3000*, each request will be routed to each nginx container in round-robin fashion.

```bash
                                                       ----------
                                                3001   | nginx  |
                                             --------------------
                                             |         | docker |
                           --------------    |         ----------
 http://localhost:3000 ->  | nmc-server | ---|         ----------
                           --------------    |         | nginx  |
                                             ---------------------
                                                3002   | docker |  
                                                       ----------
```

## Example

A sample site is provided. To test, open a terminal and type:

```bash
$ ./nmc-server --site=`pwd`/demo-site/
✓ Starting Nginx Mini-cluster...
```

In another terminal, run:

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
A mini cluster using nginx.

Usage:
  nginx-mini-cluster [flags]

Flags:
      --count string           Number of reverse proxies (default "2")
  -h, --help                   help for nginx-mini-cluster
      --nginx-version string   nginx version (default "latest")
      --port string            Initial port used by the proxies (default "3001")
      --server-port string     Server port (default "3000")
      --site string            Directory serving files (default "/home/taro/nginx-mini-cluster/site")
  -v, --version                version for nginx-mini-cluster
```