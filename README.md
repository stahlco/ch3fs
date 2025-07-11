### Overview
Distributed P2P Storage System optizimed for reads of recipes. This represents the group-work of `Cedric Borchardt`, `Johannes Duschl` 
and `Constantin Stahl` for the `TU Berlin Scalability Engineering 2025 coding task`.

---
### Setup

To build and run this system, please make sure you have following tools installed. To actually run this, please follow the instructions in section "Setup the System".

#### Runtime Dependencies

Go Environment:
   - _Go_ (I used 1.23.5)

Protocol Buffers and gRPC:
   - `protoc` - Required for automatic creation of `.pb.go` files
   - `go-grpc` plugin - to generate gRPC relevant code from `.proto` files.
     Installation:
      ```bash
      go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
      go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
      ```

Docker:
   - Docker
   - Docker Compose

just:
   - `just` for build commands, install like this:
   
   ```bash
   #macOS
   brew install just
   ```
   ```bash
   #arch linux
   sudo pacman -S just
   ```
   

#### Setup the System

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

#### Scale Cluster at Runtime
```shell
just scale [cluster | client] <amount>
```
**Example:**
Define the number of nodes, within the cluster:
```shell
just scale cluster 5
```

Define the number of clients, that access the cluster (Only for stresstesting the system):
```shell
just scale client 25
```

---
### Requirements

1. **The Application must manage some kind of state**: 

   The system persistently stores recipes using `BboltDB`. Since the architecture is optimized for read-heavy workloads, only the leader node is allowed to handle write operations. Each write is broadcast to all nodes, ensuring that every node maintains an up-to-date copy of the data. This allows any node to serve read requests independently, without requiring quorum or consensus from other nodes
 
2. **The Application needs to able to scale vertically and horizontally**: 

   The cluster is designed for easy configuration of the cluster size, which needs a architecture with focus on scalability. We achieve this leveraging `Docker Swarm` components within a `docker-compose` setup.
   It can be **scaled in our out at runtime** (see Section _Scale Cluster at Runtime_) with minimal effort of the user. This version of the system only allows horizontal scalability only on a single machine, because we use for this sandbox environment a single docker network.
   So please consider that performance may decrease on certain machines due to increased communication overhead between nodes when scaled out. 
   
   New nodes are **automatically discovered** via `hashicorp/memberlist`, which is straightforward thanks the built-in DNS service of `docker`. New nodes are seamlessly integrated into the Raft cluster.

3. **Mitigation strategies to prevent overloading componentes when scaled out**:

   Firstly, our architecture prevents that from happening. The only component which could be overloaded is our leader. But we assume that writes happen not frquently (~ A Recipe per minute).
   But we added Several Load Shedding Strategies, ranging from 
   - **Prioritized Load Shedding** where we shed writes at a certain cpu treshold.
   - **Probabilistic Load Shedding**, which sheds load (reads and writes) based on cpu treshold and probability (e.g. 80% cpu treshold, implies ~10% Load will be shedded).
   - **Deadline-aware shedding**, rejects request with a remaining context (timeout) that is not fullfillable
   - **Cache based shedding**, if the cpu treshold is to high, we reject request that lead to a cache miss
    
   We've also implemented Retries with Backoff and Jitter to distribute the request when retries are needed.
 
4. **Two more strategies from the AWS Builders Library or the lecture content**
   -  Native to are architecure we decided on a single leader based architecure. The coordination leader election we use the `raft consensus protocol` from hashicorp.
   -  We also use caching to speed up the reads and prevent the services from overhead of database lookups