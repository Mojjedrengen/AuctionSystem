to regenerate gRPC run:

```
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative grpc/auctionsystem.proto
```

# Work order:

Currently I am creating the system with only one server.
When it works I will create the replication.
I Think we should implement Leader-based replication.

- To do: 1 Leader, 1 or more followers
- Client can bid to any node
- if it isn't leader, it forwards the bid to leader
- leader updates global state
- leader replicated update to all followers
- everyone stays consistent
- if leader dies -> followers detect and elect new leader


