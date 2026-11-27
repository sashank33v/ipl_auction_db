package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrControlNotFound = errors.New("auction not found")
	ErrControlHostOnly = errors.New("only room host can control auction")
	ErrControlConflict = errors.New("auction cannot perform requested control in its current state")
)

// SetPaused serializes with bidding and settlement via the auction row lock.
// While paused, the old deadline is retained; paused_at freezes its meaning.
func (r *AuctionEngineRepository) SetPaused(ctx context.Context, auctionID, userID string, pause bool) (*Auction, error) {
	tx, ok := r.db.(pgx.Tx)
	if !ok {
		return nil, errors.New("auction control requires a transaction")
	}
	var roomID, hostID, status, roomStatus, round string
	var current, bid, bidder *string
	var started, deadline, paused *time.Time
	err := tx.QueryRow(ctx, `SELECT a.room_id,r.host_user_id,a.status,r.status,a.round_type,
        a.current_player_id,a.current_bid::text,a.current_bidder_team_id,
        a.timer_started_at,a.timer_deadline_at,a.paused_at
        FROM auctions a JOIN rooms r ON r.id=a.room_id WHERE a.id=$1 FOR UPDATE OF a`, auctionID).
		Scan(&roomID, &hostID, &status, &roomStatus, &round, &current, &bid, &bidder, &started, &deadline, &paused)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrControlNotFound
	}
	if err != nil {
		return nil, err
	}
	if hostID != userID {
		return nil, ErrControlHostOnly
	}
	var participantID string
	err = tx.QueryRow(ctx, `SELECT id FROM room_participants WHERE room_id=$1 AND user_id=$2`, roomID, userID).Scan(&participantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrControlHostOnly
	}
	if err != nil {
		return nil, err
	}
	if roomStatus != "IN_PROGRESS" || round != "MAIN" || (bid == nil) != (bidder == nil) ||
		(current == nil && (bid != nil || started != nil || deadline != nil)) ||
		(current != nil && (started == nil || deadline == nil)) {
		return nil, ErrControlConflict
	}
	// Sample DB wall time only after taking the lock and checking permissions.
	var now time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return nil, err
	}
	event := "AUCTION_PAUSED"
	if pause {
		if status != "ACTIVE" || paused != nil || (deadline != nil && !deadline.After(now)) {
			return nil, ErrControlConflict
		}
		_, err = tx.Exec(ctx, `UPDATE auctions SET status='PAUSED',paused_at=$3,last_controlled_by=$2,updated_at=$3 WHERE id=$1`, auctionID, participantID, now)
	} else {
		if status != "PAUSED" || paused == nil || now.Before(*paused) || (deadline != nil && !deadline.After(*paused)) {
			return nil, ErrControlConflict
		}
		event = "AUCTION_RESUMED"
		// Shift both timestamps by the pause duration, preserving the original
		// window size and exactly the remaining bidding time at pause.
		_, err = tx.Exec(ctx, `UPDATE auctions SET status='ACTIVE',
            timer_started_at=timer_started_at+($3::timestamptz-paused_at),
            timer_deadline_at=timer_deadline_at+($3::timestamptz-paused_at),
            paused_at=NULL,last_controlled_by=$2,updated_at=$3 WHERE id=$1`, auctionID, participantID, now)
	}
	if err != nil {
		return nil, err
	}
	// Append a durable event and outbox row in the same transaction as state.
	_, err = tx.Exec(ctx, `WITH e AS (
        INSERT INTO auction_events(room_id,auction_id,sequence_number,event_type,payload)
        SELECT a.room_id,a.id,COALESCE((SELECT MAX(sequence_number) FROM auction_events WHERE auction_id=a.id),0)+1,$2,
            jsonb_build_object('auction_id',a.id,'room_id',a.room_id,'status',a.status,
                'room_player_id',a.current_player_id,'timer_started_at',a.timer_started_at,
                'timer_deadline_at',a.timer_deadline_at,'paused_at',a.paused_at,
                'last_controlled_by',a.last_controlled_by,'reason','host_control')
        FROM auctions a WHERE a.id=$1 RETURNING id,room_id,auction_id,event_type,payload
    ) INSERT INTO outbox_events(room_id,auction_id,event_id,event_type,payload,status)
      SELECT room_id,auction_id,id,event_type,payload,'PENDING' FROM e`, auctionID, event)
	if err != nil {
		return nil, err
	}
	return NewAuctionRepository(tx).GetByID(ctx, auctionID)
}
