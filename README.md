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

#### Scale Cluster at Runtime

Includes, all `.pb.go` files. Does not include the compiled binary.

---
### Requirements

1. **The Application must manage some kind of state**
   The system stores Recipes persistent in `BboltDB`. Because this system lays heavy focus on writes we allow write only the leader of the cluster.
   Each write will be broadcasting, to all nodes. So each node holds state and allows reads without the need of quorums or consensus of other nodes.
 
2. **The Application needs to able to scale vertically and horizontally**
   The cluster can be configured with ease, due to our design and replication strategie using `docker swarm` components within `docker compose` setup.
   The cluster can easily be scaled at runtime (Section [Scale cluster](#Scale Cluster at Runtime), both in and out. 
   Based on the machine the cluster runs on, it can happen that the cluster performs worse when scaled out, due to the overhead of communication.
   New Nodes will be discovered by `hashicorp/memberlist` outmatically thanks to the `docker dns` service, and will be added to the raft cluster.
3. **Mitigation strategies to prevent overloading componentes when scaled out**
   Firstly, our architecture prevents that from happening. The only component which could be overloaded is our leader. But we assume that writes happen not frquently (~ A Recipe per minute).
   But we added Several Load Shedding Strategies, ranging from 
   - **Prioritized Load Shedding** where we shed writes at a certain cpu treshold.
   - **Probabilistic Load Shedding**, which sheds load (reads and writes) based on cpu treshold and probability (80% treshold -> 10% Load will be shedded).
   - **Deadline-aware shedding**, rejects request with a remaining context (timeout) that is not fullfillable
   - **Cache based shedding**, if the cpu treshold is to high, we reject request that lead to a cache miss
    
   We've also implemented Retries with Backoff and Jitter to distribute the request when retries are needed.
 
4. **Two more strategies from the AWS Builders Library or the lecture content**
   -  Native to are architecure we decided on a single leader based architecure. The coordination leader election we use the `raft consensus protocol` from hashicorp.
   -  We also use caching to speed up the reads and prevent the services from overhead of database lookups