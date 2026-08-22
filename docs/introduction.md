# Introduction

Realm is a simplistic orchestration service where the source of truth is just a configuration file.

## Configuration file

A realm config file specifies the nodes of a cluster and the loads that can be deployed on each node. That's the source of truth of your cluster, and the idea here is to simplify operations with means like GitOps.

Lets start with a basic configuration.

```yaml
nodes:
  lab1:
    url: http://192.168.1.1:9000
    driver: linux

loads:
  hello:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/library/hello-world:latest
```

This config specify that our cluster is composed by just one node `lab1` that uses the `linux` node driver, and this node is running a realm agent on `http://192.168.1.1:9000` (more on agents later).

It also specify a `hello` load that uses the `container` driver with a basic hello-world image settings. This load is meant to be deployed on the `lab1` node.

So to summarize the basic primitive:

* nodes: different cluster nodes where a realm agent is running
* loads: loads meant to be deployed and control per each node

It's important to clarify that Realm providers of several drivers for each one of these primitives. For example, it provides a driver for Linux machines but also Windows machines or even VirtualMachines. For loads, it provides for example drivers to handle containers or native processes.

Now with this configuration you can command your cluster using the realm CLI:

```shell
realm n state lab1
```

## Agents

As we already introduced, realm can be run as an agent, it's meant to be daemonized and it handles requests from the CLI by means of a REST API.




