### Overview
Distributed P2P Storage System for Recipes. This represents the Group-work of Cedric Borchardt, Johannes Duschl 
and Constantin Stahl for the TU Berlin Scalability Engineering 2025 coding task.
---
### Setup

Clone the repository

To generate the .proto files:
```shell
just proto
```

To run the project (includes the building process):
```shell
just run
```

To set `up` the system (represents `docker compose up --detach`):

```
just <not implemented yet>
```

---
### Technologies

Go Library for Membership in a Dist Sys via Gossiping: [Link](https://github.com/hashicorp/memberlist)\
CRDTs: [Link](https://crdt.tech/) \
Mögliche Database, jeder Peer hat eine eigene: [Link](https://github.com/etcd-io/bbolt) \
Raft Consensus Algorithm: [Link](https://raft.github.io/) \
Auch von HashiCorp eine Implementation von Raft: [Link](https://github.com/hashicorp/raft)