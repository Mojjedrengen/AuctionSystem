to regenerate gRPC run:

```
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative grpc/auctionsystem.proto
```

- To do: 
Election Algo

How to run:
Open Terminal #1

cd AuctionSystem/auction
go run main.go

You will be prompted with 'leaderAdress'
Press Enter to leave it empty (since this is the leader)

Example input from first terminal / Node:
LeaderAdress: Press enter to leave empty
Leaderadress Replication: 6000
Port to listen to: 50051
Port to listen to (replication): 6000

You should then see:
Auction server running on port 50051
is leader


Open Terminal #2
cd AuctionSystem/auction
go run main.go

Fill in:

leaderAdress: localhost:50051
leaderAdress Replication: localhost:6000
port to listen to: 50052
port to listen to (replication):6001

You should then see:
Auction server running on port 50052
bidTimeFrame:30  bidStartTime:1764096263  isBitOngoin:true <nil>


You now have both a Leader and a follower node.


Terminal #3 - Client:
cd AuctionSystem/client
go run main.go -addr=localhost:50051

You should see something like:
client connected
bid response:  ack:SUCCESS
result:  highestbidder:{value:"client-1"}

Once the leader is down (Just force him down if need be), then client can connect to the follower as the follower has made itself the leader, you can do so with:

go run main.go -addr=localhost:50052 (Or whatever you put its port to listen to)
