package server

import (
	"context"
	"sort"
	"sync"

	auctionsystem "github.com/Mojjedrengen/AuctionSystem/grpc"
)

type AuctionServer struct {
	auctionsystem.UnimplementedAuctionServer

	isLeader      bool
	id            uint64
	highestBidder *auctionsystem.UUID
	highestBid    uint64
	knownBidders  []*auctionsystem.UUID
	bidTimeframe  uint32
	bidStartTime  uint64
	mu            sync.Mutex
	lastWonBidder *auctionsystem.UUID
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
	s.knownBiddersInsertContains(amount.Id)
	var ack auctionsystem.Ackmsg
	s.mu.Lock()
	defer s.mu.Unlock()
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
