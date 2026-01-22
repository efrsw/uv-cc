# Video

https://drive.google.com/file/d/1nFY3es8KK2aOKsHythdlvh2MuqPZ1IYw/view?usp=sharing

# Build
## Installation

I've provided binaries in the repo, but in case the system has to be rebuilt, one can use the `make build` command, which will produce 2 executables inside the `./bin` directory. The `bucket` executable is a storage implementation and `client` is a CLI tool that can be used to do required operations.

The system can be started by launching multiple bucket processes with different indices:

```bash
./bucket -idx=0 -baseport=47073 -nodes=4 -datadir=./data
./bucket -idx=1 -baseport=47073 -nodes=4 -datadir=./data
./bucket -idx=2 -baseport=47073 -nodes=4 -datadir=./data
./bucket -idx=3 -baseport=47073 -nodes=4 -datadir=./data
```

Additionally, `Makefile` includes commands to start the system (`make start-buckets`), stop it (`make stop-buckets`) or generate the test report (`make report`). The log file from report can be inspected [here](./src/report.log).

[Each bucket creates a subdirectory `chunk_{idx}`](./src/bucket/main.go#L36-41) inside the data directory containing its snapshot file, WAL file, and log file.

# Architecture
## Shared elements

### Directory implementation

[Directory implementation](./src/internal/directory/directory.go) is the first place where tradeoffs happen. As the assignment states that it is not necessary to implement a dynamic directory (i.e. no addition/deletion of nodes), I chose a hybrid approach between what Dynamo uses and modulo hashing. Essentially, in my current implementation, [I create an in-memory view of topology that is stored on each node](./src/internal/directory/directory.go#L27-31). This gives an obvious advantage that each node can be used as a query coordinator by forwarding requests with a single redirect. Additionally, I avoided creating a SPOF by not creating a separate "directory" process, which contributes to the system's availability.

The directory is constructed at startup based on configuration flags. Each node knows the base port, the total number of nodes, and its own index. From this information, it can construct URLs for all other nodes in the topology (`http://localhost:{baseport+i}/` for i in 0..nodes-1). This information is sufficient to route any request to the correct bucket.

Obviously, the decision of using modulo routing was made under the assumption of static topology. If the topology were to change, my current implementation would have two major issues. The first one is that in case of bucket addition/deletion, my system would need to rehash **all** the data. To mitigate this, consistent hashing could be used (then only the data from adjacent buckets must be rehashed). The second issue is that there's no way of dynamically updating the directory. If I'd like to have more availability, I would need to implement some gossip protocol to communicate topology changes at runtime.

### Logging implementation

The [logging system](./src/internal/logging/logger.go) uses Go's structured logging package `log/slog` with plain text formatting. [Each bucket writes to its own log file](./src/bucket/main.go#L46) (`bucket_{idx}.log`) to avoid concurrent write conflicts. The logger includes the bucket ID in every log entry for traceability.

Log entries capture several types of events:

- **Lifecycle events**: `startup` (with data file and WAL file paths)
- **Storage operations**: `insert`, `delete`, `search` (with key and file information)
- **Coordination events**: `proxy_insert`, `proxy_delete`, `proxy_search` (with target bucket)
- **Range query operations**: `range_start`, `range_query`, `range_response`, `range_complete` (tracking the fan-out process)
- **Compaction events**: `compaction_start`, `compaction_complete` (with keys written count)

All entries are timestamped in RFC3339 format, which allows the merge command to reconstruct a global ordering of events across all buckets.

## Bucket architecture

[Bucket](./src/bucket/main.go) is a part of the application that implements the actual storage of the files. It is fully distributed and doesn't follow a single leader in order to coordinate requests.

The bucket can be configured using multiple [flags](./src/bucket/main.go#L24-27):

- `idx` to determine the self position in the directory
- `datadir` to determine the location of the output files (i.e. the snapshot, the write-ahead log (WAL) and the log file)
- `baseport` to determine the starting port of the whole topology (defaults to 47073, derived from my student ID)
- `nodes` to determine the total amount of nodes in topology

### Coordinator selection

During implementation I considered multiple possible architectures. Initially, I was going to simply use a centralized balancer to coordinate the requests from the client. If this choice was made, it would lead to creating a single point of failure, which would make the system less available. 

Therefore, to avoid creating SPOF, I decided to go with a simplified version of Dynamo's design: each node is an executable which stores the whole directory in memory. This means that each node can act as a coordinator, even if the requested key lies outside the range it is responsible for.

There are multiple caveats to this design choice I had to address. The most obvious one was the handling of range queries. In my current implementation, range query triggers fan-out requests to all other nodes. One problem here is that we have to distinguish if the request came from the client or from another node. If the request comes from another node, we should not propagate it further (otherwise the nodes will enter a self-amplifying loop of sending requests). To solve this, [I use the `X-From-Proxy` header](./src/internal/storage/kv_store_proxy.go#L119) to signal that a request is already being coordinated and should [not trigger further fan-out](./src/bucket/handlers/bucket_handler.go#L114-120).

### Storage implementation

The storage layer went through several iterations before settling on the current design. The first implementation used a naive approach: [loading the entire database from a JSON file on startup](https://gitta-lab.cs.univie.ac.at/cc25w/a12347073/-/commit/01342fe7b1f1589d879cc764f52da933f812fb59#7aabaed9e0a10d5d0052440f5c339c67b2e383a0), keeping everything in memory during operation, and writing the complete state back to JSON on each modification. This obviously incurred severe performance problems, as each insert or delete would trigger a full database write to disk.

In order to solve this problem I considered using multiple strategies. At first, I was thinking of using an append-only log file which then could be used to restore the snapshot by replaying all events that happened. The problem with this approach was high startup time, because replaying potentially millions of operations would make the system unusable after crashes.

Finally, I settled on WAL + eventual compaction process. This hybrid approach gives us the benefits of both worlds: fast writes and reasonable recovery time. Here's how it works in practice:

On each insertion or deletion, the operation is [first written to the append-only write-ahead log file](./src/internal/storage/kv_store_distributed.go#L82-99). This is a fast operation since we're just appending text. To ensure durability the write is [immediately synced to disk](./src/internal/storage/kv_store_distributed.go#L92). Then the [in-memory cache is updated to reflect the change](./src/internal/storage/kv_store_distributed.go#L105-107).

The compaction process runs under two conditions: [when the WAL file exceeds 1MB in size](./src/internal/storage/kv_store_distributed.go#L98), or [every 15 seconds](./src/internal/storage/kv_store_distributed.go#L307) (whichever comes first). During compaction, [we take a snapshot of the current in-memory state](./src/internal/storage/kv_store_distributed.go#L279-283) and [write it atomically to the JSON snapshot file](./src/internal/fs/json.go#L56). Then we [truncate the WAL and reset the size counter](./src/internal/storage/kv_store_distributed.go#L292-301). I chose these thresholds based on balancing crash recovery time (smaller WAL means faster replay) against compaction overhead (too frequent compaction wastes CPU).

The atomic write pattern is crucial for crash safety. Instead of writing directly to the snapshot file, [we write to a temporary file first](./src/internal/fs/json.go#L62), [sync it to disk](./src/internal/fs/json.go#L76), and then [atomically rename it to replace the old snapshot](./src/internal/fs/json.go#L84). This ensures that we never have a corrupted snapshot file: either the old version exists, or the new version exists, but never a half-written file.

On startup, [the system loads the snapshot from JSON into memory](./src/internal/storage/kv_store_distributed.go#L321), then [replays any operations in the WAL that happened after the last compaction](./src/internal/storage/kv_store_distributed.go#L339). This gives us both durability and performance.

### Concurrency handling

Since Go's HTTP server handles requests concurrently, I had to carefully manage concurrent access to shared state. The storage uses multiple locks for different purposes:

- `storageMu` ([RWMutex](./src/internal/storage/kv_store_distributed.go#L26)) protects the in-memory cache. Read operations (search, range) take a read lock allowing concurrent reads, while writes (insert, delete) take an exclusive write lock.
- `walMu` ([Mutex](./src/internal/storage/kv_store_distributed.go#L30)) protects the WAL file and the size counter, ensuring that WAL writes are atomic and size tracking is consistent.
- `compactMu` ([Mutex](./src/internal/storage/kv_store_distributed.go#L38)) prevents multiple compaction processes from running simultaneously, which would corrupt the snapshot file.

This lock granularity allows good concurrency as multiple searches can run in parallel, and WAL writes don't block reads from the cache.

### HTTP API design

Each bucket exposes a REST-like HTTP API with the following endpoints:

- `POST /values` - [Insert a key-value pair](./src/bucket/handlers/bucket_handler.go#L30) (body: JSON with `{key: uint, value: string}`)
- `DELETE /values/{key}` - [Delete a key-value pair](./src/bucket/handlers/bucket_handler.go#L31)
- `GET /values/{key}` - [Search for a specific key](./src/bucket/handlers/bucket_handler.go#L32) (returns the value as plain text)
- `GET /values?k1={start}&k2={end}` - [Range query](./src/bucket/handlers/bucket_handler.go#L33) (returns JSON array of key-value pairs)

The API uses appropriate HTTP status codes: 200 for success, 404 for key not found, 400 for bad requests. This makes the system easy to test with standard HTTP tools like curl.

### Graceful shutdown

The bucket process listens for interrupt signals (SIGINT, SIGTERM) and performs graceful shutdown. When a signal is received, [the HTTP server stops accepting new requests and waits for existing requests to complete (with a 30-second timeout)](./src/bucket/main.go#L106). Then we [trigger a final compaction](./src/bucket/main.go#L111) to [flush any uncommitted WAL entries to the snapshot](./src/internal/storage/kv_store_distributed.go#L251), ensuring no data loss on clean shutdown. Only after the storage is properly closed does the process exit.

## Client architecture

Client is a CLI application that essentially provides a type-safe interface over the 4 base commands: `insert`, `delete`, `search` and `range`. It does so by reusing the shared implementation of the [proxied client](./src/internal/storage/kv_store_proxy.go) ([that is also used when other buckets coordinate requests that were not supposed for them](./src/internal/storage/kv_store_distributed.go#L71-80)). Client has multiple subcommands that should be explained in greater detail:

### Shared flags

There are multiple flags that are shared by all client subcommands, as some logic is shared. These flags are `-baseport` and `-nodes`. They are used to determine the starting port of topology and the amount of nodes in it. Client [determines a random coordinator](./src/client/main.go#L230) for each subcommand in order to show that each node is capable of executing the request and acting as a coordinator. The random selection uses Go's `crypto/rand` package to ensure uniform distribution across buckets.

### Insert

[Insert subcommand](./src/client/main.go#L34-59) is used to add a new value to the storage. It accepts two flags: `-key` which is a 64-bit uint and `-value` which is a string value that the user would like to add. The write is idempotent, meaning that the value will be upserted into the storage. This behavior goes in line with the Dynamo implementation and is needed to avoid rewrite conflicts.

As already described, under the hood the `insert` operation determines a random coordinator node to process the request and then sends a `POST` HTTP request with key and value as JSON payload to the `/values` endpoint.

### Delete

[Delete subcommand](./src/client/main.go#L61-85) is used to delete the value from the storage by its key. It accepts `-key` argument (also 64-bit uint) and also does a coordinator choice. Then it executes a `DELETE` HTTP request to the `/values/{key}` endpoint to remove a value from the store. As in the case of the `insert` operation, delete is idempotent (also by the same logic as in Dynamo case). Therefore, if the value was not present in the storage, then nothing happens.

### Search

[Search subcommand](./src/client/main.go#L87-112) is used to query the storage for a given key. Unlike Insert and Delete operations, it can actually return an error. So, the flags for the `search` operation are the common ones and `-key` (which is obviously a 64-bit uint). 

As with all commands in the client, it randomly calculates the coordinator node and sends a GET request to `/values/{key}` to obtain the stored value. When the value is not in the storage, the server returns HTTP 404, which the client translates to an error message.

### Range

[Range subcommand](./src/client/main.go#L114-146) executes a range query, given the two CLI arguments: `-start` and `-end`. It then returns an array of values whose keys satisfy `start <= key <= end`. There was an interesting design choice in the making of this subcommand: instead of fanning out requests from the client, [I let the underlying storage implementation do this](./src/client/main.go#L140). The coordinator bucket then actually fans out requests to other nodes in the topology with the `X-From-Proxy: true` header, signaling that no further fan-out is needed. Results are then merged into a single slice and returned to the user [sorted by key](./src/bucket/handlers/bucket_handler.go#L126-128).

### Upload

[Upload subcommand](./src/client/main.go#L180-222) is used to parse and upload a CSV file with records to the storage. It takes `-file` and `-max` flags:

- `-file` is used to specify the `.csv` file which should be uploaded. It should come in a predetermined format (example file lies in the `./data` directory).
- `-max` flag specifies the total amount of MB that will be transferred in the upcoming run. When the limit is reached, the upload stops.

Under the hood, this subcommand triggers the [load function that executes a provided callback on each line read](./src/internal/csvloader/loader.go#L121-123). In my implementation, [I've combined coordinator selection and the `Insert` function of the `ProxyStorage` as a callback](./src/client/main.go#L194-201). Therefore, each row is inserted using a new coordinator. I understand that it is not the most efficient way to do so, as executing a random function on each call imposes significant overhead on the writing process. It would be better to select a single coordinator for a batch upload, but, in my opinion, that would be out of scope for the assignment.

The upload process provides live progress feedback by tracking bytes read and displaying megabytes uploaded. The CSV loader is specifically designed for the [MyAnimeList dataset](./data/reviews_100m.csv) structure, parsing records with [fields like uid, profile, anime_uid, text, score, and link](./src/internal/csvloader/loader.go#L76-81).

### Merge

[Merge subcommand](./src/client/main.go#L148-178) is used to produce the final log report of the run. It stands apart from all other commands as it doesn't initiate any HTTP request to the system. The arguments are: `-dir` to specify the directory to scan for `.log` files and `-output` to specify the file the report should be written to.

The merge process walks [the directory tree recursively](./src/client/main.go#L256), finds all `.log` files, [parses the RFC3339 timestamps from each log line](./src/client/main.go#L329-336), and [sorts all entries chronologically across all buckets](./src/client/main.go#L169-171). This produces a unified timeline of all operations across the distributed system, which is essential for understanding system behavior and satisfying the assignment's logging requirements.

# Discussion
## Hash function discussion

I've used Go's [FNV-1a hash function](./src/internal/storage/kv_store_distributed.go#L417) for my implementation primarily because it is fast and stable across runs. 

The hash function implementation converts the uint key to bytes using [little-endian encoding](./src/internal/storage/kv_store_distributed.go#L419), computes the FNV-1a hash, and then [takes modulo of the number of buckets to determine placement](./src/internal/storage/kv_store_distributed.go#L68). In my opinion, this approach (FNV1a + modulo) is sufficient for this assignment's requirements of static topology where we don't need the minimal-rehashing properties of consistent hashing.

## Event-driven architecture discussion

Events could be the same as my WAL entries, as they are essentially events. For an event-driven architecture, I would suggest having a Kafka topic that contains all the events. Storage then would simply need to listen to these events and apply them. Each dictionary operation would publish an event:

- `InsertEvent(key, value, timestamp)` for insertions
- `DeleteEvent(key, timestamp)` for deletions  
- `SearchEvent(key)` for searches (though this might not need persistence)
- `RangeQueryEvent(start, end)` for range queries

Buckets would subscribe to events in their key range and apply them to local storage. This decouples request handling from storage updates and enables features like event replay for recovery or analytics.

In my current WAL-based implementation, the log entries already follow this pattern:
- `INSERT:12345::some_value`
- `DELETE:67890`

These could be directly mapped to Kafka messages. The main difference would be that instead of each bucket maintaining its own WAL, all events would go to a shared Kafka topic partitioned by key range. Buckets would consume their partition and maintain their local state. This would enable better scaling and replication, though at the cost of additional infrastructure complexity.

## CAP theorem discussion

I think the CAP theorem might be viewed considering multiple architectural choices that I've made. The distributed nature of the system means that data is partitioned across multiple nodes — each bucket is responsible for a subset of the keyspace determined by the hash function. This partitioning enables horizontal scaling.

However, my current implementation without replication actually sacrifices both availability and partition tolerance for consistency. When a bucket fails, all keys mapped to that bucket become permanently unavailable until the bucket recovers (there's no replica to serve the data). This makes the system **CP** in CAP terms, prioritizing consistency and partition tolerance, though "partition tolerance" is somewhat limited since we lose data access (not data itself) during partitions.

More specifically, here's how my implementation handles the CAP tradeoffs:

**Consistency**: The system maintains strong consistency. Each key exists in exactly one bucket, and all operations on that key go through the same bucket. [The WAL ensures durability, because once a write returns success, it's committed to disk and will survive crashes](./src/internal/storage/kv_store_distributed.go#L92). There's no possibility of stale reads or conflicting writes since there's no replication.

**Availability**: This is where the system makes sacrifices. If a bucket process crashes or becomes unreachable, approximately 1/N of the keyspace becomes unavailable (where N is the number of buckets). The coordinator pattern helps somewhat: if one bucket is down but is not responsible for the requested key, the system can still route to the correct bucket. However, there's no redundancy for actual data storage.

**Partition tolerance**: The system can handle network partitions between buckets to some extent. If bucket A can't reach bucket B, operations on keys owned by A still work fine. However, operations requiring data from B will fail. Range queries are particularly vulnerable since they require all buckets to respond.

In my opinion, my system was designed in a way that extending it to be highly-available is not that hard. There's no single point of failure in the coordination layer, and each node [can coordinate requests](./src/internal/storage/kv_store_distributed.go#L178-188). So, replication similar to the one used in Dynamo (when each entry has a list of preferred nodes) could make my system more available at the cost of eventual consistency. We would need to implement read repair, hinted handoff, and vector clocks to handle conflicts.

Given that the system is used as a logistics system, I think that prioritizing consistency over availability was indeed the right choice. We cannot afford double-spending of inventory, so the data must always be consistent. It's better to reject an operation during a partial failure than to allow inventory inconsistencies that could lead to overselling products.
