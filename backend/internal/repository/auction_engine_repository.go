package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// AuctionEngineRepository coordinates existing workflows using one auction lock.
// Only the main pool is automated in this milestone.
type AuctionEngineRepository struct{ db DBTX }

func NewAuctionEngineRepository(db DBTX) *AuctionEngineRepository {
	return &AuctionEngineRepository{db: db}
}

// Hold a short result-display interval before revealing another player.
const engineDue = `a.status='ACTIVE' AND a.round_type='MAIN' AND r.status='IN_PROGRESS' AND (
 (a.current_player_id IS NOT NULL AND a.timer_deadline_at <= clock_timestamp()) OR
 (a.current_player_id IS NULL AND a.updated_at + INTERVAL '3 seconds' <= clock_timestamp() AND EXISTS (
 SELECT 1 FROM room_players rp WHERE rp.room_id=a.room_id AND rp.status='AVAILABLE' AND rp.auction_sequence IS NOT NULL
 )))`

func (r *AuctionEngineRepository) DueAuctions(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT a.id FROM auctions a JOIN rooms r ON r.id=a.room_id WHERE `+engineDue+` ORDER BY a.updated_at,a.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Advance performs at most one transition. A second worker or a bid holding the
// same row lock causes a harmless skip; deadlines are rechecked after locking.
func (r *AuctionEngineRepository) Advance(ctx context.Context, auctionID string) (string, error) {
	tx, ok := r.db.(pgx.Tx)
	if !ok {
		return "", errors.New("auction engine requires a transaction")
	}
	var roomID, hostID string
	var current, bid, team *string
	err := tx.QueryRow(ctx, `SELECT a.room_id,r.host_user_id,a.current_player_id,a.current_bid::text,a.current_bidder_team_id
 FROM auctions a JOIN rooms r ON r.id=a.room_id
 WHERE a.id=$1 AND `+engineDue+` FOR UPDATE OF a SKIP LOCKED`, auctionID).Scan(&roomID, &hostID, &current, &bid, &team)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var due bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auctions a JOIN rooms r ON r.id=a.room_id WHERE a.id=$1 AND `+engineDue+`)`, auctionID).Scan(&due); err != nil {
		return "", err
	}
	if !due {
		return "", nil
	}
	if current == nil {
		var next string
		err = tx.QueryRow(ctx, `SELECT id FROM room_players WHERE room_id=$1 AND status='AVAILABLE' AND auction_sequence IS NOT NULL ORDER BY auction_sequence,id LIMIT 1`, roomID).Scan(&next)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		_, err = NewMainNominationWorkflowRepository(tx).NominatePlayer(ctx, MainNominationParams{AuctionID: auctionID, PlayerID: next, UserID: hostID})
		if err != nil {
			return "", err
		}
		return "PLAYER_NOMINATED", nil
	}
	if (bid == nil) != (team == nil) {
		return "", errors.New("inconsistent winning bid state")
	}
	payload := map[string]any{"auction_id": auctionID, "room_id": roomID, "room_player_id": *current}
	if bid != nil {
		payload["team_id"] = *team
		payload["purchase_price"] = *bid
		payload["status"] = "SOLD"
	} else {
		payload["status"] = "UNSOLD"
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	if bid != nil {
		_, err = NewAuctionSaleRepository(tx).CompleteSale(ctx, CompleteSaleParams{AuctionID: auctionID, EventPayload: raw})
		if err != nil {
			return "", err
		}
		return "PLAYER_SOLD", nil
	}
	_, err = NewAuctionUnsoldRepository(tx).MarkUnsold(ctx, MarkUnsoldParams{AuctionID: auctionID, EventPayload: raw})
	if err != nil {
		return "", err
	}
	return "PLAYER_UNSOLD", nil
}

// RecoverOnStartup prevents a process restart from automatically settling a player.
// Run before serving HTTP. Single backend process deployment is required for now.
func (r *AuctionEngineRepository) RecoverOnStartup(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `UPDATE auctions SET status='RECOVERING', updated_at=statement_timestamp() WHERE status='ACTIVE'`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

var (
	ErrRecoveryNotFound = errors.New("auction not found")
	ErrRecoveryHostOnly = errors.New("only room host can resume recovery")
	ErrRecoveryConflict = errors.New("auction is not a recoverable main auction")
)

// ResumeRecovered requires the host and preserves the current player and bid.
// Downtime is not counted as bidding time: grant a fresh full window on resume.
func (r *AuctionEngineRepository) ResumeRecovered(ctx context.Context, auctionID, userID string) error {
	tx, ok := r.db.(pgx.Tx)
	if !ok {
		return errors.New("recovery requires a transaction")
	}
	var host, status, roomStatus, round, roomID string
	var participantID string
	var current, bid, team *string
	err := tx.QueryRow(ctx, `SELECT r.host_user_id,a.status,r.status,a.round_type,a.room_id,a.current_player_id,a.current_bid::text,a.current_bidder_team_id
 FROM auctions a JOIN rooms r ON r.id=a.room_id WHERE a.id=$1 FOR UPDATE OF a`, auctionID).Scan(&host, &status, &roomStatus, &round, &roomID, &current, &bid, &team)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRecoveryNotFound
	}
	if err != nil {
		return err
	}
	if host != userID {
		return ErrRecoveryHostOnly
	}
	if status != "RECOVERING" || roomStatus != "IN_PROGRESS" || round != "MAIN" {
		return ErrRecoveryConflict
	}
	if (bid == nil) != (team == nil) || (current == nil && bid != nil) {
		return fmt.Errorf("%w: inconsistent bid state", ErrRecoveryConflict)
	}
	// last_controlled_by references room_participants.id, not users.id.
	if err = tx.QueryRow(ctx, `SELECT id FROM room_participants WHERE room_id=$1 AND user_id=$2`, roomID, userID).Scan(&participantID); errors.Is(err, pgx.ErrNoRows) {
		return ErrRecoveryHostOnly
	} else if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE auctions SET status='ACTIVE', last_controlled_by=$2, paused_at=NULL,
 timer_started_at=CASE WHEN current_player_id IS NULL THEN NULL ELSE statement_timestamp() END,
 timer_deadline_at=CASE WHEN current_player_id IS NULL THEN NULL WHEN current_bid IS NULL THEN statement_timestamp()+INTERVAL '15 seconds' ELSE statement_timestamp()+INTERVAL '7 seconds' END,
 updated_at=statement_timestamp() WHERE id=$1`, auctionID, participantID)
	if err != nil {
		return err
	}
	// Record recovery as a durable sequenced event and outbox entry.
	_, err = tx.Exec(ctx, `WITH e AS (
 INSERT INTO auction_events(room_id,auction_id,sequence_number,event_type,payload)
 SELECT a.room_id,a.id,COALESCE((SELECT MAX(sequence_number) FROM auction_events WHERE auction_id=a.id),0)+1,'AUCTION_RESUMED',
 jsonb_build_object('auction_id',a.id,'room_id',a.room_id,'room_player_id',a.current_player_id,'timer_deadline_at',a.timer_deadline_at,'reason','restart_recovery')
 FROM auctions a WHERE a.id=$1 RETURNING id,room_id,auction_id,event_type,payload
 ) INSERT INTO outbox_events(room_id,auction_id,event_id,event_type,payload,status) SELECT room_id,auction_id,id,event_type,payload,'PENDING' FROM e`, auctionID)
	return err
}
