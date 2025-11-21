package main

import (
	"fmt"
	"log"
	"net"

	auctionsystem "github.com/Mojjedrengen/AuctionSystem/grpc"
	"github.com/Mojjedrengen/AuctionSystem/server"

	"google.golang.org/grpc"
)

func main() {
	server := server.NewAuctionServer(
		1,
		30,
		true,
	)

	go server.BidManager()

	grpcServer := grpc.NewServer()

	auctionsystem.RegisterAuctionServer(grpcServer, server)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failted to listen: %v", err)
	}
	fmt.Println("Auction server running on port 50051")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failted to server %v", err)
	}
}
