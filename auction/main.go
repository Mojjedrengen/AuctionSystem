package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	auctionsystem "github.com/Mojjedrengen/AuctionSystem/grpc"
	"github.com/Mojjedrengen/AuctionSystem/server"

	"google.golang.org/grpc"
)

func main() {
	var leaderAdress string
	var leaderAdressRep string
	var port uint16
	var portRep uint16

	fmt.Println("leaderAdress")
	fmt.Scanln(&leaderAdress)
	fmt.Println(leaderAdress)
	fmt.Println("leaderAdress Replication")
	fmt.Scanln(&leaderAdressRep)
	fmt.Println(leaderAdressRep)
	fmt.Println("port to listen to")
	fmt.Scanln(&port)
	fmt.Println(port)
	fmt.Println("port to listen to (replication)")
	fmt.Scanln(&portRep)
	fmt.Println(portRep)
	isLeader := true

	if leaderAdress != "" {
		isLeader = false
	}

	repServer, server := server.NewReplicationServer(
		uint64(rand.Int()),
		30,
		isLeader,
		leaderAdress,
		leaderAdressRep,
		portRep,
	)

	go server.BidManager()

	grpcServer := grpc.NewServer()

	auctionsystem.RegisterAuctionServer(grpcServer, server)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failted to listen: %v", err)
	}
	fmt.Printf("Auction server running on port %d\n", port)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failted to server %v", err)
		}
	}()
	time.Sleep(10 * time.Second)
	if !isLeader {
		fmt.Println(repServer.GetLeader().Fetch(context.Background(), &auctionsystem.Self{}))
	} else {
		fmt.Println("is leader")
	}
	for {
	}
	//fmt.Println(repServer.Fetch(context.Background(), &auctionsystem.Self{}))
}

func (s *AuctionServer) LeaderMonitor() {
	if s.isLeader {
		return
	}

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := s.leaderHeartbeat.Heartbeat(ctx, &auctionsystem.Self{Id: s.id})
		cancel()

		if err != nil {
			fmt.Println("heartbeat fail, leader might be dead", err)
			s.promotoToLeader()
			return
		}

		time.Sleep(1 * time.Second)
	}
}
