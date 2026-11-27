package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sashank33v/ipl_auction_db/backend/internal/auction"
	"github.com/sashank33v/ipl_auction_db/backend/internal/database"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

// Run against a disposable, migrated PostgreSQL database only.
func TestLobbyLifecycleIntegration(t *testing.T) {
	url := os.Getenv("IPL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set IPL_TEST_DATABASE_URL to a migrated disposable database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var token [6]byte
	if _, err := rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	prefix := hex.EncodeToString(token[:])
	var masterCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM players WHERE is_active AND is_auction_eligible`).Scan(&masterCount); err != nil {
		t.Fatal(err)
	}
	if masterCount != auction.FantasyPoolSize {
		t.Fatalf("seed disposable DB with approved 543-player pool; got %d", masterCount)
	}
	var users []string
	var roomID string
	defer func() {
		clean, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if roomID != "" {
			for _, q := range []string{"DELETE FROM outbox_events WHERE room_id=$1", "DELETE FROM auction_events WHERE room_id=$1", "DELETE FROM bids WHERE room_id=$1", "DELETE FROM squads WHERE room_id=$1", "DELETE FROM auctions WHERE room_id=$1", "DELETE FROM teams WHERE room_id=$1", "DELETE FROM room_players WHERE room_id=$1", "DELETE FROM room_participants WHERE room_id=$1", "DELETE FROM rooms WHERE id=$1"} {
				if _, err := pool.Exec(clean, q, roomID); err != nil {
					t.Errorf("fixture cleanup: %v", err)
				}
			}
		}
		for _, id := range users {
			if _, err := pool.Exec(clean, "DELETE FROM users WHERE id=$1", id); err != nil {
				t.Errorf("fixture cleanup: %v", err)
			}
		}
	}()
	for i := 0; i < 6; i++ {
		name := prefix + string(rune('a'+i))
		var id string
		if err := pool.QueryRow(ctx, `INSERT INTO users(username,email,password_hash) VALUES($1,$2,'test-only') RETURNING id`, name, name+"@example.invalid").Scan(&id); err != nil {
			t.Fatal(err)
		}
		users = append(users, id)
	}
	tm := database.NewTransactionManager(pool)
	rr := repository.NewRoomRepository(pool)
	pr := repository.NewRoomParticipantRepository(pool)
	tr := repository.NewTeamRepository(pool)
	ar := repository.NewAuctionRepository(pool)
	rs := NewRoomService(tm, rr)
	ps := NewRoomParticipantService(tm, pr, rr)
	ts := NewTeamService(tm, tr, rr, pr)
	as := NewAuctionService(tm, ar, rr, ps, ts)
	room := &repository.Room{RoomCode: strings.ToUpper(prefix), HostUserID: users[0], Name: "Lobby test", Status: "WAITING", MaxParticipants: 4}
	if err := rs.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}
	roomID = room.ID
	if _, err := as.CreateAuction(ctx, roomID); err != nil {
		t.Fatal(err)
	}
	if _, err := ps.JoinRoom(ctx, roomID, users[0]); !errors.Is(err, ErrInvalidRoomCode) {
		t.Fatalf("UUID-only join must fail: %v", err)
	}
	if _, err := ps.JoinRoom(ctx, room.RoomCode, users[0]); err != nil {
		t.Fatal(err)
	}
	// Five simultaneous attempts for three remaining slots: exactly three succeed.
	var wg sync.WaitGroup
	results := make(chan error, 5)
	for _, id := range users[1:] {
		wg.Add(1)
		go func(id string) { defer wg.Done(); _, err := ps.JoinRoom(ctx, room.RoomCode, id); results <- err }(id)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 3 {
		t.Fatalf("successful concurrent joins = %d; want 3", success)
	}
	members, err := ps.GetRoomParticipants(ctx, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 4 {
		t.Fatalf("members=%d; want 4", len(members))
	}
	if _, err := as.StartAuction(ctx, roomID, users[0]); err == nil {
		t.Fatal("auction started before franchise selection")
	}
	codes := []string{"CSK", "MI", "RCB", "SRH"}
	var outsider string
	for i, member := range members {
		team := &repository.Team{RoomID: roomID, ParticipantID: member.ID, FranchiseCode: codes[i]}
		if err := ts.CreateTeam(ctx, team); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range users {
		found := false
		for _, m := range members {
			if m.UserID == id {
				found = true
			}
		}
		if !found {
			outsider = id
			break
		}
	}
	if outsider == "" {
		t.Fatal("missing outsider fixture")
	}
	if visible, err := rs.GetRoomForUser(ctx, roomID, outsider); err != nil || visible != nil {
		t.Fatalf("outsider room read: %v %v", visible, err)
	}
	if _, err := as.StartAuction(ctx, roomID, outsider); err == nil {
		t.Fatal("non-host started auction")
	}
	rps := NewRoomPlayerService(tm, repository.NewRoomPlayerRepository(pool), rr, repository.NewPlayerRepository(pool))
	if _, err := rps.PrepareAuctionQueue(ctx, roomID, outsider, auction.QueueOptions{}); !errors.Is(err, ErrQueueHostOnly) {
		t.Fatalf("outsider queue: %v", err)
	}
	// Exercise automatic generation without requiring a preview request.
	if err := tm.WithinTransaction(ctx, func(tx pgx.Tx) error {
		if _, err := repository.NewRoomRepository(tx).GetByIDForUpdate(ctx, roomID); err != nil {
			return err
		}
		_, err := repository.NewRoomPlayerRepository(tx).EnsureAuctionQueue(ctx, roomID, auction.QueueOptions{}, false)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	opts, err := auction.ValidateQueueOptions(auction.QueueOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Put uncapped batters first, even before capped categories.
	opts.CategoryOrder[0], opts.CategoryOrder[5] = opts.CategoryOrder[5], opts.CategoryOrder[0]
	opts.ShuffleCategories = []string{"UNCAPPED_BATTER"}
	preview, err := rps.PrepareAuctionQueue(ctx, roomID, users[0], opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview) != auction.FantasyPoolSize {
		t.Fatalf("queue count %d", len(preview))
	}
	if preview[0].AuctionCategory != "UNCAPPED_BATTER" {
		t.Fatalf("custom order first category: %s", preview[0].AuctionCategory)
	}
	seenCategories := map[string]bool{}
	previousCategory := ""
	for i, p := range preview {
		if p.AuctionCategory != previousCategory {
			if seenCategories[p.AuctionCategory] {
				t.Fatal("category split across queue")
			}
			seenCategories[p.AuctionCategory] = true
			previousCategory = p.AuctionCategory
		}
		if p.AuctionSequence == nil || *p.AuctionSequence != i+1 {
			t.Fatalf("sequence at %d: %v", i, p.AuctionSequence)
		}
	}
	started, err := as.StartAuction(ctx, roomID, users[0])
	if err != nil {
		t.Fatal(err)
	}
	after, err := rps.GetPlayersByRoom(ctx, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(preview) {
		t.Fatal("queue size changed")
	}
	for i, p := range after {
		if p.ID != preview[i].ID {
			t.Fatalf("prepared order changed at %d", i)
		}
	}
	if _, err := rps.PrepareAuctionQueue(ctx, roomID, users[0], auction.QueueOptions{}); !errors.Is(err, repository.ErrQueueConflict) {
		t.Fatalf("active queue mutation: %v", err)
	}
	if _, err := NewMainNominationWorkflowService(tm).NominatePlayer(ctx, repository.MainNominationParams{AuctionID: started.ID, PlayerID: preview[1].ID, UserID: users[0]}); err == nil || !strings.Contains(err.Error(), "next available queued player") {
		t.Fatalf("out-of-order nomination: %v", err)
	}
	if started.Status != "ACTIVE" {
		t.Fatalf("auction status=%s", started.Status)
	}
	locked, err := rs.GetRoomByID(ctx, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if locked.Status != "IN_PROGRESS" {
		t.Fatalf("room status=%s", locked.Status)
	}
	if _, err := ps.JoinRoom(ctx, room.RoomCode, outsider); !errors.Is(err, ErrRoomClosed) {
		t.Fatal("outsider joined locked room")
	}
	if _, err := ps.JoinRoom(ctx, room.RoomCode, users[0]); err != nil {
		t.Fatalf("existing host rejoin: %v", err)
	}
	if _, err := as.StartAuction(ctx, roomID, users[0]); err == nil {
		t.Fatal("auction started twice")
	}

	// Engine tests use DB-controlled time shifts instead of sleeping through timers.
	engine := NewAuctionEngineService(tm, repository.NewAuctionEngineRepository(pool))
	changeTime := func(sql string) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, started.ID); err != nil {
			t.Fatal(err)
		}
	}
	advance := func(want string) {
		t.Helper()
		got, err := engine.Advance(ctx, started.ID)
		if err != nil || got != want {
			t.Fatalf("advance: got %q err=%v want %q", got, err, want)
		}
	}
	read := func() *repository.Auction {
		t.Helper()
		a, err := ar.GetByID(ctx, started.ID)
		if err != nil || a == nil {
			t.Fatalf("read auction: %v", err)
		}
		return a
	}
	checkWindow := func(want time.Duration) {
		t.Helper()
		a := read()
		if a.TimerStartedAt == nil || a.TimerDeadlineAt == nil || a.TimerDeadlineAt.Sub(*a.TimerStartedAt) != want {
			t.Fatalf("wrong deadline: %+v", a)
		}
	}
	changeTime(`UPDATE auctions SET updated_at=clock_timestamp()-INTERVAL '4 seconds' WHERE id=$1`)
	advance("PLAYER_NOMINATED")
	first := read()
	if first.CurrentPlayerID == nil || *first.CurrentPlayerID != preview[0].ID {
		t.Fatal("wrong first nomination")
	}
	checkWindow(15 * time.Second)
	advance("")
	changeTime(`UPDATE auctions SET timer_deadline_at=clock_timestamp()-INTERVAL '1 second' WHERE id=$1`)
	advance("PLAYER_UNSOLD")
	advance("") // display interval: no immediate nomination
	changeTime(`UPDATE auctions SET updated_at=clock_timestamp()-INTERVAL '4 seconds' WHERE id=$1`)
	advance("PLAYER_NOMINATED")
	second := read()
	if second.CurrentPlayerID == nil || *second.CurrentPlayerID != preview[1].ID {
		t.Fatal("wrong second nomination")
	}
	teams, err := tr.GetByRoom(ctx, roomID)
	if err != nil || len(teams) != 4 {
		t.Fatalf("teams: %v", err)
	}
	var requestID string
	if err := pool.QueryRow(ctx, `SELECT gen_random_uuid()`).Scan(&requestID); err != nil {
		t.Fatal(err)
	}
	bidParams := repository.AcceptBidParams{AuctionID: started.ID, RoomPlayerID: preview[1].ID, TeamID: teams[0].ID, BidAmount: preview[1].BasePrice, RequestID: requestID, EventPayload: []byte(`{}`)}
	accepted, err := NewBidWorkflowService(tm).AcceptBid(ctx, bidParams)
	if err != nil || accepted == nil || accepted.CreatedAt.IsZero() {
		t.Fatalf("accepted bid timestamp: %+v %v", accepted, err)
	}

	// Snapshot is consistent with the accepted bid and the event watermark.
	realtime := repository.NewRealtimeRepository(pool)
	snap, _, err := realtime.Pull(ctx, started.ID, users[0], nil, true)
	if err != nil || snap == nil || snap.Auction.CurrentBid == nil || snap.CurrentPlayer == nil || len(snap.Teams) != 4 || snap.LastEvent == nil || snap.LastEvent.EventType != "BID_ACCEPTED" {
		t.Fatalf("live snapshot: %+v %v", snap, err)
	}
	if _, _, err := realtime.Pull(ctx, started.ID, outsider, nil, true); !errors.Is(err, repository.ErrRealtimeForbidden) {
		t.Fatalf("outsider snapshot: %v", err)
	}
	firstCursor := snap.Sequence
	same, _, err := realtime.Pull(ctx, started.ID, users[0], &firstCursor, false)
	if err != nil || same != nil {
		t.Fatalf("unchanged cursor: %v %v", same, err)
	}
	// Same request ID for a different team must not be treated as a valid retry.
	wrong := bidParams
	wrong.TeamID = teams[1].ID
	if _, err := NewBidWorkflowService(tm).AcceptBid(ctx, wrong); err == nil {
		t.Fatal("conflicting bid ID accepted")
	}
	if retry, err := NewBidWorkflowService(tm).AcceptBid(ctx, bidParams); err != nil || retry == nil || retry.ID != accepted.ID || !retry.CreatedAt.Equal(accepted.CreatedAt) {
		t.Fatalf("same bid retry: %+v %v", retry, err)
	}
	checkWindow(7 * time.Second)
	// A valid higher bid from another franchise must reset the rolling window.
	changeTime(`UPDATE auctions SET timer_deadline_at=clock_timestamp()+INTERVAL '4 seconds' WHERE id=$1`)
	bidParams.TeamID = teams[1].ID
	if err := pool.QueryRow(ctx, `SELECT gen_random_uuid(), ($1::numeric+0.10)::text`, bidParams.BidAmount).Scan(&bidParams.RequestID, &bidParams.BidAmount); err != nil {
		t.Fatal(err)
	}
	if _, err := NewBidWorkflowService(tm).AcceptBid(ctx, bidParams); err != nil {
		t.Fatal(err)
	}

	checkWindow(7 * time.Second)
	newer, replayed, err := realtime.Pull(ctx, started.ID, users[0], &firstCursor, false)
	if err != nil || newer == nil || len(replayed) != 1 || replayed[0].EventType != "BID_ACCEPTED" || newer.Sequence != firstCursor+1 {
		t.Fatalf("event replay: %+v %v %v", newer, replayed, err)
	}
	advance("")
	// A server restart must not settle even an expired timer.
	changeTime(`UPDATE auctions SET timer_deadline_at=clock_timestamp()-INTERVAL '1 second' WHERE id=$1`)
	if _, err := engine.RecoverOnStartup(ctx); err != nil {
		t.Fatal(err)
	}
	if read().Status != "RECOVERING" {
		t.Fatal("startup did not protect auction")
	}
	advance("")
	if err := as.ResumeRecoveredAuction(ctx, started.ID, outsider); !errors.Is(err, repository.ErrRecoveryHostOnly) {
		t.Fatalf("outsider resume: %v", err)
	}
	if err := as.ResumeRecoveredAuction(ctx, started.ID, users[0]); err != nil {
		t.Fatal(err)
	}
	checkWindow(7 * time.Second)
	if a := read(); a.CurrentBid == nil || a.CurrentBidderTeamID == nil || *a.CurrentBidderTeamID != bidParams.TeamID {
		t.Fatal("recovery lost bid")
	}
	changeTime(`UPDATE auctions SET timer_deadline_at=clock_timestamp()-INTERVAL '1 second' WHERE id=$1`)
	if err := pool.QueryRow(ctx, `SELECT gen_random_uuid()`).Scan(&bidParams.RequestID); err != nil {
		t.Fatal(err)
	}
	if _, err := NewBidWorkflowService(tm).AcceptBid(ctx, bidParams); err == nil || !strings.Contains(err.Error(), "deadline expired") {
		t.Fatalf("late bid: %v", err)
	}
	// Concurrent expiry attempts may settle only once.
	var settlements sync.WaitGroup
	outcomes := make(chan string, 8)
	failures := make(chan error, 8)
	for i := 0; i < 8; i++ {
		settlements.Add(1)
		go func() {
			defer settlements.Done()
			got, err := engine.Advance(ctx, started.ID)
			outcomes <- got
			failures <- err
		}()
	}
	settlements.Wait()
	close(outcomes)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	sold := 0
	for result := range outcomes {
		if result == "PLAYER_SOLD" {
			sold++
		} else if result != "" {
			t.Fatalf("unexpected transition %s", result)
		}
	}
	if sold != 1 {
		t.Fatalf("settlements=%d want 1", sold)
	}
	var purchases int
	var balanceOK bool
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM squads WHERE room_id=$1 AND room_player_id=$2`, roomID, preview[1].ID).Scan(&purchases); err != nil || purchases != 1 {
		t.Fatalf("purchases=%d err=%v", purchases, err)
	}
	if err := pool.QueryRow(ctx, `SELECT remaining_purse=120-$2::numeric FROM teams WHERE id=$1`, bidParams.TeamID, bidParams.BidAmount).Scan(&balanceOK); err != nil || !balanceOK {
		t.Fatalf("purse deduction mismatch: %v", err)
	}
	var events, outbox int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM auction_events WHERE auction_id=$1 AND event_type='PLAYER_SOLD'),(SELECT count(*) FROM outbox_events WHERE auction_id=$1 AND event_type='PLAYER_SOLD')`, started.ID).Scan(&events, &outbox); err != nil || events != 1 || outbox != 1 {
		t.Fatalf("sale event/outbox counts=%d/%d err=%v", events, outbox, err)
	}
}
