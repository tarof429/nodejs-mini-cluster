# NGINX Mini-Cluster

## What's this?

This application performs round-robin load balancing between two nginx docker containers. Load balancing is performed by implementing a RoundRobinHandler to handle each request. Docker containers are pulled directly from docker hub using docker APIs. 