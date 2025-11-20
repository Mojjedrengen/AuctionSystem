to regenerate gRPC run:

```
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative grpc/auctionsystem.proto
```

# Work order:

Currently I am creating the system with only one server.
When it works I will create the replication.
I Think we should implement Leader-based replication.
