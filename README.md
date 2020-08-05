# NGINX Mini-Cluster

NGINX Mini-Cluster is a CLI application that runs Nginx docker containers as reverse proxies behind the main HTTP server. Each container is used in round-robin fashion. If a container becomes inaccessible it will be restarted automatically. 

By default, the server port (the port clients should connect to) is set to 3000. 

```bash
$ ./nmc-server --help
A mini cluster using nginx.

Usage:
  nginx-mini-cluster [flags]

Flags:
      --count string           Number of reverse proxies (default "2")
  -h, --help                   help for nginx-mini-cluster
      --nginx-version string   nginx version (default "latest")
      --port string            Initial port used by the proxies (default "3001")
      --server-port string     Server port (default "3000")
      --site string            Directory serving files (default "<pwd>/site")
  -v, --version                version for nginx-mini-cluster

$ ./nmc-server
✓ Starting Nginx Mini-cluster...
2020/08/04 21:17:58 Forwarding request to port: 3002
2020/08/04 21:18:02 Forwarding request to port: 3001
2020/08/04 21:18:37 Forwarding request to port: 3002
2020/08/04 21:18:37 Handling error for 3002

2020/08/04 21:18:37 Re-creating proxy...
2020/08/04 21:18:37 Stopping container
2020/08/04 21:18:37 Container could not be stopped
2020/08/04 21:18:37 Creating container
2020/08/04 21:18:37 Starting container
2020/08/04 21:18:38 Proxy available
2020/08/04 21:18:42 Forwarding request to port: 3001
2020/08/04 21:18:43 Forwarding request to port: 3002
```