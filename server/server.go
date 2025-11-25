package server

import (
	"context"
	"errors"
	"log"
	"sort"
	"sync"
	"time"

	auctionsystem "github.com/Mojjedrengen/AuctionSystem/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type AuctionServer struct {
	auctionsystem.UnimplementedAuctionServer

	isLeader      bool
	leaderAddr    string
	bidChan       chan auctionsystem.Ackmsg
	leaderConn    *grpc.ClientConn
	leader        auctionsystem.AuctionClient
	id            uint64
	highestBidder *auctionsystem.UUID
	highestBid    uint64
	knownBidders  []*auctionsystem.UUID
	bidTimeframe  uint32
	bidStartTime  uint64
	mu            sync.Mutex
	lastWonBidder *auctionsystem.UUID
	isBitOngoin   bool
	state         auctionsystem.State
}

func NewAuctionServer(id uint64, bidTimeframe uint32, isLeader bool, leaderAddr string) *AuctionServer {
	s := &AuctionServer{
		isLeader:      isLeader,
		leaderAddr:    leaderAddr,
		bidChan:       make(chan auctionsystem.Ackmsg),
		id:            id,
		highestBidder: nil,
		highestBid:    0,
		knownBidders:  make([]*auctionsystem.UUID, 0),
		bidTimeframe:  bidTimeframe,
		bidStartTime:  uint64(time.Now().Unix()),
		lastWonBidder: nil,
		isBitOngoin:   true,
		state:         auctionsystem.State_ONGOING,
	}
	if !isLeader {
		var opts []grpc.DialOption
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		conn, err := grpc.NewClient(leaderAddr, opts...)
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
