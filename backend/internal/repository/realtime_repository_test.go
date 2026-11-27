package repository

import "testing"

func TestContiguousEvents(t *testing.T) {
	if !ContiguousEvents([]AuctionEvent{{SequenceNumber: 3}, {SequenceNumber: 4}}, 2, 4) {
		t.Fatal("valid replay rejected")
	}
	if ContiguousEvents([]AuctionEvent{{SequenceNumber: 3}, {SequenceNumber: 5}}, 2, 4) {
		t.Fatal("gap accepted")
	}
	if ContiguousEvents([]AuctionEvent{{SequenceNumber: 3}}, 2, 4) {
		t.Fatal("missing event accepted")
	}
}
func TestBidRequestIdentity(t *testing.T) {
	b := &Bid{TeamID: "team", RoomPlayerID: "player", BidAmount: "2.00"}
	p := AcceptBidParams{TeamID: "team", RoomPlayerID: "player", BidAmount: "2"}
	if _, e := matchExistingBid(b, p); e != nil {
		t.Fatal(e)
	}
	p.TeamID = "other"
	if _, e := matchExistingBid(b, p); e == nil {
		t.Fatal("different team accepted")
	}
	p.TeamID = "team"
	p.BidAmount = "2.10"
	if _, e := matchExistingBid(b, p); e == nil {
		t.Fatal("different price accepted")
	}
}
