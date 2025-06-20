### Overview
Distributed P2P Storage System optizimed for reads of recipes. This represents the group-work of `Cedric Borchardt`, `Johannes Duschl` 
and `Constantin Stahl` for the `TU Berlin Scalability Engineering 2025 coding task`.

---
### Setup

Before starting the system, please create the `.proto` files.

```shell
just proto
```
Those files will be moved into the container at start up.
As well, you can use this command to use the generated Stubs for the coding-process.

To set `up` the system (represents `docker compose up`):

```shell
just up
```
This spin's up x replicas, where x is defined in the docker compose. For now x = 3.
All containers are in the same network (called `kitchen`) which will also be created in the `up`-process.
Thanks to the network, we can communicate in between containers, by just calling the **container-name**.

#### Shut down

To shut `down` the system (`docker compose down`):
```shell
just down
```
Removes all containers and the network, also those created by a previous run that are no longer defined in the current `docker-compose.yaml`.
Has a shutdown timeout of 3 seconds.


#### Cleaning the Project
To remove all generated files:
```shell
just clean
```

Includes, all `.pb.go` files. Does not include the compiled binary.

---
### Technologies

Go Library for Membership in a Dist Sys via Gossiping: [Github-Link](https://github.com/hashicorp/memberlist)\
CRDTs: [Blogpost-Link](https://crdt.tech/) \
Mögliche Database, jeder Peer hat eine eigene: [Github-Link](https://github.com/etcd-io/bbolt) \
Raft Consensus Algorithm: [Github-Link](https://raft.github.io/) \
Auch von HashiCorp eine Implementation von Raft: [Github-Link](https://github.com/hashicorp/raft)