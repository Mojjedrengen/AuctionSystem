package main

import (
	"context"
	"fmt"
	"log"
	"time"

	auctionsystem "github.com/Mojjedrengen/AuctionSystem/grpc"
	"google.golang.org/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to %v", err)
	}
	defer conn.Close()

	client := auctionsystem.NewAuctionClient(conn)
	fmt.Println("client connected")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	bidResponse, err := client.Bid(ctx, &auctionsystem.Amount{
		Amount: 10,
		Id:     &auctionsystem.UUID{Value: "client-1"},
	})
	if err != nil {
		log.Fatalf("bid err: %v", err)
	}

	fmt.Println("bid response: ", bidResponse)

	resultResponse, err := client.Result(ctx, &emptypb.Empty{})
	if err != nil {
		log.Fatalf("result error: %v", err)
	}

	fmt.Println("result: ", resultResponse)
}
