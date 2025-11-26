package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sort"
	"sync"
	"time"

	auctionsystem "github.com/Mojjedrengen/AuctionSystem/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type ReplicationServer struct {
	auctionsystem.UnimplementedReplicationSyncServer

	leaderAddr      string
	leaderConn      *grpc.ClientConn
	leader          auctionsystem.ReplicationSyncClient
	self            *AuctionServer
	selfAdress      string
	leaderHeartbeat auctionsystem.ReplicationSyncClient
	cluster         map[uint64]string
}

func (s *ReplicationServer) RegisterNode(ctx context.Context, req *auctionsystem.Self) (*emptypb.Empty, error) {
	if !s.self.isLeader {
		return nil, errors.New("not leader")
	}

	s.cluster[req.Id] = req.Address

	fmt.Println("node registered: ", req.Id, "at", req.Address)

	return &emptypb.Empty{}, nil
}

func (s *ReplicationServer) GetCluster(ctx context.Context, req *auctionsystem.Self) (*auctionsystem.ClusterInfo, error) {
	//	if !s.self.isLeader {
	//		return nil, errors.New("not leader")
	//	}

	result := &auctionsystem.ClusterInfo{
		Members: make(map[uint64]string),
	}

	for id, addr := range s.cluster {
		result.Members[id] = addr
	}
	result.Members[s.self.id] = s.selfAdress

	return result, nil
}

type AuctionServer struct {
	auctionsystem.UnimplementedAuctionServer

	selfAddress            string
	isLeader               bool
	leaderAddr             string
	bidChan                chan auctionsystem.Ackmsg
	leaderConn             *grpc.ClientConn
	leader                 auctionsystem.AuctionClient
	id                     uint64
	highestBidder          *auctionsystem.UUID
	highestBid             uint64
	knownBidders           []*auctionsystem.UUID
	bidTimeframe           uint32
	bidStartTime           uint64
	mu                     sync.Mutex
	lastWonBidder          *auctionsystem.UUID
	isBitOngoin            bool
	state                  auctionsystem.State
	selfReplicationAddress string
	//leaderHeartbeat        auctionsystem.ReplicationSyncClient
}

func (s *AuctionServer) ApplySnapshot(data *auctionsystem.AuctionData) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.highestBid = data.HighestBid
	s.highestBidder = data.Highestbidder
	s.knownBidders = data.KnownBidders
	s.bidTimeframe = data.BidTimeFrame
	s.bidStartTime = data.BidStartTime
	s.lastWonBidder = data.LastWonBidder
	s.isBitOngoin = data.IsBitOngoin
	s.state = data.State
}

func (s *ReplicationServer) UpdateCluser(cluser *auctionsystem.ClusterInfo) {
	for id, address := range cluser.GetMembers() {
		if _, ok := s.cluster[id]; !ok {
			s.cluster[id] = address
		}
	}
}

func (s *AuctionServer) promotoToLeader() {

	fmt.Println("Node: ", s.id, " is now promoting itself to leader")

	s.isLeader = true

	if s.leaderConn != nil {
		s.leaderConn.Close()
		s.leaderConn = nil
	}

	s.leader = nil
}

func (s *ReplicationServer) promotoToLeader() {
	for id, addr := range s.cluster {
		var opts []grpc.DialOption
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		conn, err := grpc.NewClient(addr, opts...)
		if err != nil {
			log.Printf("Failed to attempt to accent with %v during connection: %v", id, err)
		}
		client := auctionsystem.NewReplicationSyncClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		ack, err := client.AccensionAttempt(ctx, &auctionsystem.ExtendedSelf{RepSelf: &auctionsystem.Self{
			Address: s.selfAdress,
			Id:      s.self.id,
		}, AuctionAddress: s.self.selfAddress})
		if err == nil {
			if ack.Ack != auctionsystem.Ack_SUCCESS {
				conn.Close()
				cancel()
				return
			}
		}
		conn.Close()
		cancel()
	}

	// is now leader
	s.leaderAddr = s.selfAdress
	s.leaderConn.Close()
	s.leaderConn = nil
	s.leader = nil
	s.leaderHeartbeat = nil
	s.self.promotoToLeader()
}
func (s *ReplicationServer) AccensionAttempt(ctx context.Context, them *auctionsystem.ExtendedSelf) (*auctionsystem.Ackmsg, error) {
	highest := s.self.id

	for otherID := range s.cluster {
		if otherID > highest {
			highest = otherID
		}
	}
	if them.RepSelf.Id > highest {
		if _, exist := s.cluster[them.RepSelf.Id]; !exist {
			s.cluster[them.RepSelf.Id] = them.RepSelf.Address
		}
		s.leaderAddr = them.RepSelf.Address
		var opts []grpc.DialOption
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		conn, err := grpc.NewClient(them.RepSelf.Address, opts...)
		if err != nil {
			log.Printf("Failed to connect to leader during startup: %v", err)
		}
		s.leaderConn = conn
		s.leader = auctionsystem.NewReplicationSyncClient(conn)
		s.leaderHeartbeat = auctionsystem.NewReplicationSyncClient(conn)

		s.self.isLeader = false

		s.self.selfAddress = them.AuctionAddress
		conn2, err2 := grpc.NewClient(them.AuctionAddress, opts...)
		if err2 != nil {
			log.Printf("Failed to connect to leader during startup: %v", err)
		}
		s.self.leaderConn.Close()
		s.self.leaderConn = conn2
		s.self.leader = auctionsystem.NewAuctionClient(conn2)

		return &auctionsystem.Ackmsg{Ack: auctionsystem.Ack_SUCCESS}, nil
	} else {
		return &auctionsystem.Ackmsg{Ack: auctionsystem.Ack_FAIL}, nil
	}
}

func (s *ReplicationServer) LeaderMonitor() {
	if s.self.isLeader {
		return
	}

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := s.leaderHeartbeat.Heartbeat(ctx, &auctionsystem.Self{Id: s.self.id})
		cancel()

		if err != nil {
			fmt.Println("heartbeat fail, leader might be dead", err)
			//	cluster, err2 := s.leaderHeartbeat.GetCluster(context.Background(), &auctionsystem.Self{Id: s.self.id})
			//	if err2 != nil {
			//		fmt.Println("could not get cluster", err2)
			//		s.self.promotoToLeader()
			//		return
			//	}

			for id, addr := range s.cluster {
				var opts []grpc.DialOption
				opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
				conn, err := grpc.NewClient(addr, opts...)
				if err != nil {
					log.Printf("Failed to sync cluster with %d during connection: %v", id, err)
				}
				client := auctionsystem.NewReplicationSyncClient(conn)
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				cluser, err := client.GetCluster(ctx, &auctionsystem.Self{Id: s.self.id})
				if err != nil {
					log.Printf("Failed to sync cluster with %d during getCluster: %v", id, err)
				}
				s.UpdateCluser(cluser)
				conn.Close()
				cancel()
			}

			highest := s.self.id

			for otherID := range s.cluster {
				if otherID > highest {
					highest = otherID
				}
			}

			if highest == s.self.id {
				fmt.Println("Promoting self to leader")
				s.promotoToLeader()
			} else {
				fmt.Println("I'm not highest ID, waiting for other node to become leader")
			}
			return
		}

		data, err2 := s.leaderHeartbeat.Fetch(context.Background(), &auctionsystem.Self{Id: s.self.id})
		if err2 == nil {
			s.self.ApplySnapshot(data)
		}
		cluster, err3 := s.leaderHeartbeat.GetCluster(context.Background(), &auctionsystem.Self{Id: s.self.id})
		if err3 == nil {
			s.UpdateCluser(cluster)
		}

		time.Sleep(1 * time.Second)
	}
}

func (s *ReplicationServer) GetLeader() auctionsystem.ReplicationSyncClient {
	return s.leader
}

func NewReplicationServer(id uint64, bidTimeframe uint32, isLeader bool, leaderAddr string, leaderAddrReplication string, AuctionAddress string, port uint16) (*ReplicationServer, *AuctionServer) {
	s := &ReplicationServer{}
	s.cluster = make(map[uint64]string)

	selfAddr := fmt.Sprintf("localhost:%d", port)
	s.selfAdress = selfAddr

	s.self = NewAuctionServer(id, bidTimeframe, isLeader, leaderAddr, leaderAddrReplication, selfAddr, AuctionAddress)

	if !isLeader {

		var opts []grpc.DialOption
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		conn, err := grpc.NewClient(leaderAddrReplication, opts...)
		if err != nil {
			log.Fatalf("Failed to connect to leader during startup: %v", err)
		}
		s.leaderConn = conn
		s.leader = auctionsystem.NewReplicationSyncClient(conn)
		s.leaderHeartbeat = auctionsystem.NewReplicationSyncClient(conn)

		_, err = s.leaderHeartbeat.RegisterNode(
			context.Background(),
			&auctionsystem.Self{
				Address: s.self.selfReplicationAddress,
				Id:      s.self.id,
			},
		)
		if err != nil {
			log.Println("error registering node with leader: ", err)
		}
	}
	s.selfAdress = fmt.Sprintf("localhost:%d", port)
	lis, err := net.Listen("tcp", s.selfAdress)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption

	grpcServer := grpc.NewServer(opts...)
	auctionsystem.RegisterReplicationSyncServer(grpcServer, s)
	go grpcServer.Serve(lis)
	go s.LeaderMonitor()
	return s, s.self
}

func NewAuctionServer(id uint64, bidTimeframe uint32, isLeader bool, leaderAddr string, leaderAddrReplication string, selfReplicationAddress string, selfAddress string) *AuctionServer {
	s := &AuctionServer{
		isLeader:               isLeader,
		leaderAddr:             leaderAddr,
		selfAddress:            selfAddress,
		bidChan:                make(chan auctionsystem.Ackmsg),
		id:                     id,
		highestBidder:          nil,
		highestBid:             0,
		knownBidders:           make([]*auctionsystem.UUID, 0),
		bidTimeframe:           bidTimeframe,
		bidStartTime:           uint64(time.Now().Unix()),
		lastWonBidder:          nil,
		isBitOngoin:            true,
		state:                  auctionsystem.State_ONGOING,
		selfReplicationAddress: selfReplicationAddress,
	}
	if !isLeader {

		var opts []grpc.DialOption
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		conn, err := grpc.NewClient(leaderAddrReplication, opts...)
		if err != nil {
			log.Fatalf("Failed to connect to leader during startup: %v", err)
		}
		s.leaderConn = conn
		s.leader = auctionsystem.NewAuctionClient(conn)
	}
	return s
}

func (s *AuctionServer) knownBiddersInsert(UUID *auctionsystem.UUID) {
	s.mu.Lock()
	s.knownBidders = append(s.knownBidders, UUID)
	sort.Slice(s.knownBidders, func(i, j int) bool {
		return s.knownBidders[i].Value < s.knownBidders[j].Value
	})
	s.mu.Unlock()
}

func (s *AuctionServer) knownBiddersContains(UUID *auctionsystem.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	start := 0
	end := len(s.knownBidders) - 1
	for start <= end {
		mid := (start + end) / 2
		if s.knownBidders[mid] == UUID {
			return true
		} else if s.knownBidders[mid].Value < UUID.Value {
			start = mid + 1
		} else if s.knownBidders[mid].Value > UUID.Value {
			end = mid - 1
		}
	}
	return false
}

func (s *AuctionServer) knownBiddersInsertContains(UUID *auctionsystem.UUID) {
	if !s.knownBiddersContains(UUID) {
		s.knownBiddersInsert(UUID)
	}
}

func (s *AuctionServer) Bid(ctx context.Context, amount *auctionsystem.Amount) (*auctionsystem.Ackmsg, error) {
	if !s.isLeader {
		s.mu.Lock()
		defer s.mu.Unlock()
		bidResponse, err := s.leader.Bid(ctx, amount)
		if err != nil {
			// TODO: probs add election start here
			log.Printf("leader conn err on bid: %v", err)
			return nil, errors.New("Failed to connect to leader")
		}
		return bidResponse, nil
	}
	s.knownBiddersInsertContains(amount.Id)
	var ack auctionsystem.Ackmsg
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != auctionsystem.State_ONGOING {
		exceptmsg := "Bid is done. Please bid when new bid begins"
		return &auctionsystem.Ackmsg{
			Ack:       auctionsystem.Ack_EXCEPTION,
			Exception: &exceptmsg,
		}, errors.New("No currently active bids")
	}
	if amount.GetAmount() > s.highestBid {
		s.highestBid = amount.GetAmount()
		s.highestBidder = amount.GetId()
		ack = auctionsystem.Ackmsg{
			Ack: auctionsystem.Ack_SUCCESS,
		}
		return &ack, nil
	} else {
		return &auctionsystem.Ackmsg{
			Ack: auctionsystem.Ack_FAIL,
		}, nil
	}
}

func (s *AuctionServer) Result(ctw context.Context, in *emptypb.Empty) (*auctionsystem.Resultmsg, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == auctionsystem.State_DONE {
		return &auctionsystem.Resultmsg{
			Highestbidder: s.lastWonBidder,
			State:         s.state,
		}, nil
	} else {
		return &auctionsystem.Resultmsg{
			Highestbidder: s.highestBidder,
			State:         s.state,
		}, nil
	}
}

func (s *AuctionServer) BidManager() {
	for {
		s.mu.Lock()
		switch s.state {
		case auctionsystem.State_DONE:
			s.highestBidder = nil
			s.highestBid = 0
			s.bidStartTime = uint64(time.Now().Unix())
			s.state = auctionsystem.State_ONGOING
		case auctionsystem.State_ONGOING:
			if s.bidStartTime-uint64(time.Now().Unix())+uint64(s.bidTimeframe) <= 0 {
				s.lastWonBidder = s.highestBidder
				s.state = auctionsystem.State_DONE
			}
		}
		s.mu.Unlock()
	}
}

func ServerManager(s *AuctionServer, grpc grpc.ServiceRegistrar) {
	go s.BidManager()

}

func (s *ReplicationServer) Fetch(ctx context.Context, nonLeader *auctionsystem.Self) (*auctionsystem.AuctionData, error) {
	s.self.mu.Lock()
	defer s.self.mu.Unlock()
	if !s.self.isLeader {
		return nil, errors.New("Is not leader")
	} else {
		return &auctionsystem.AuctionData{
			Highestbidder: s.self.highestBidder,
			HighestBid:    s.self.highestBid,
			KnownBidders:  s.self.knownBidders,
			BidTimeFrame:  s.self.bidTimeframe,
			BidStartTime:  s.self.bidStartTime,
			LastWonBidder: s.self.lastWonBidder,
			IsBitOngoin:   s.self.isBitOngoin,
			State:         s.self.state,
		}, nil
	}
}

func (s *ReplicationServer) Heartbeat(ctx context.Context, req *auctionsystem.Self) (*emptypb.Empty, error) {
	if !s.self.isLeader {
		return nil, errors.New("not the leader")
	}
	return &emptypb.Empty{}, nil
}
