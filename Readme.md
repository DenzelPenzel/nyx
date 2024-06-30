# NYX

NYX is a high-speed, experimental key-value database.
NYX provides reliable storage for your critical data, ensuring constant availability.
It is an ideal lightweight, distributed kvs data store for developers and operators alike.

## Setup

To use the template, run the following command(s):

1. [Download](https://go.dev/doc/install) or upgrade to `golang 1.19`.

2. Install all project golang dependencies by running `go mod download`.

## To Run

1. Compile NYX to machine binary by running the following project level command(s):
    * Using Make: `make build-app`

2. To run the compiled binary, you can use the following project level command(s):
    * Using Make: `make run-app`
    * Direct Call: `./bin/nyx`

3. To test local KVS instance, open the new terminal console and run Netcat.
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

## Key features

**Persistent storage**:

- New records are written to disk
- Each record has a minimum overhead of 8 bytes
- It allocates space in 2^N and attempts to reuse space if the value grows
- Allow to reuse space from deleted or evicted records

**Developer-Friendly**:

- Straightforward TCP/UDP protocol

**Large data set support**:

- Works well, even when managing multi-GB data sets

**Easy Backups**

**Support memcache text protocol**

**Provide support for load and correctness testing client package**

## Contributing

Nyx is an open source project, and contributions are gladly welcomed!
To submit your changes please check pull request rules and open a pull request.








