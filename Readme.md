# NYX

NYX is a high-speed, experimental key-value database designed for reliable, persistent storage and constant
availability.

It’s lightweight, scalable, and perfect for distributed environments where performance and fault tolerance
are key.

## Key Features

- **High Performance:** Optimized for fast read and write operations.
- **Persistent Storage:** Efficient disk writes with minimal overhead.
- **Fault Tolerance:** Keeps running even if some nodes fail.
- **Distributed Consensus:** Uses Raft to maintain a consistent state across nodes, ensuring that every change made to
  the system is made to a quorum, or none at all.
- **Scalability:** Easily add or restart nodes without extra configuration.
- **Flexible Configuration:** Customize unique node IDs and network addresses.
- **Multiple Protocols:** Supports TCP/UDP, HTTP API, and memcache text protocol.
- **Large Data Support:** Efficiently handles multi-GB data sets.
- **Built-In Testing Tools:** Includes utilities for load and correctness testing.

## Setup

To use the template, run the following command(s):

1. [Download](https://go.dev/doc/install) or upgrade to `golang 1.19`.

2. Install all project golang dependencies by running `go mod download`.

## Build and Run

1. Compile NYX to machine binary by running the following project level command(s):
    * Using Make: `make build-app`

2. To run the compiled binary, you can use the following project level command(s):
    * Using Make: `make run-app`
    * Direct Call: `./bin/nyx`

3. To test local key-value-storage instance, open the new terminal console and run Netcat.
   ```bash
   $ nc localhost 4001
   > get abc
   END
   > set abc 0 0 5
   > hello
   STORED
   > get abc
   VALUE abc 0 5
   hello
   END
   > delete abc
   DELETED
   > get abc
   END
   ```

## Cluster Configuration

NYX supports distributed clusters. Each node requires a unique ```-node-id``` along with designated HTTP and Raft
addresses. Nodes join the cluster by connecting to the leader.

### Example Cluster Setup

Assume you have three hosts: `host1`, `host2`, and `host3`.

Start the Leader Node on host1:

```
host1:$ nyx -node-id 1 -http-addr host1:4001 -raft-addr host1:4002 ~/nyx-node
```

Join a Node on host2:

```
host2:$ nyx -node-id 2 -http-addr host2:4001 -raft-addr host2:4002 -join http://host1:4001 ~/nyx-node
```

Join a Node on host3:

```
host3:$ nyx -node-id 3 -http-addr host3:4001 -raft-addr host3:4002 -join http://host1:4001 ~/nyx-node
```

Nodes automatically rejoin the cluster on restart. Join requests for nodes already in the cluster are safely ignored.

## Handling Failures and Growing the Cluster

- Node Failures: If a node crashes, simply restart it. A three-node cluster tolerates the failure of one node, including
  the leader.

- Scaling Up: Add new nodes at any time by assigning a new unique node ID and having it join the cluster

## Contributing

Nyx is an open source project, and contributions are gladly welcomed!
To submit your changes please check pull request rules and open a pull request.
