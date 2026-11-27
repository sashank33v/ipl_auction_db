package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/auction"
)

var ErrQueueConflict = errors.New("auction queue conflict")

// EnsureAuctionQueue must run inside the transaction holding the WAITING room row lock.
// Explicit preparation replaces the order; starting preserves an already valid order.
func (r *RoomPlayerRepository) EnsureAuctionQueue(ctx context.Context, roomID string, opts auction.QueueOptions, regenerate bool) (int, error) {
	if _, ok := r.db.(pgx.Tx); !ok {
		return 0, errors.New("queue requires a transaction")
	}
	opts, err := auction.ValidateQueueOptions(opts)
	if err != nil {
		return 0, err
	}
	// Freeze master writes during preparation/validation. Room lock serializes room mutations.
	if _, err = r.db.Exec(ctx, `LOCK TABLE players IN SHARE MODE`); err != nil {
		return 0, fmt.Errorf("lock master pool: %w", err)
	}
	var total, invalid int
	err = r.db.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE auction_category IS NULL OR NOT (auction_category = ANY($1::text[])) OR trim(country)='' OR upper(country)='UNVERIFIED' OR is_overseas <> (lower(country)<>'india')) FROM players WHERE is_active AND is_auction_eligible`, opts.CategoryOrder).Scan(&total, &invalid)
	if err != nil {
		return 0, err
	}
	if total != auction.FantasyPoolSize || invalid != 0 {
		return 0, fmt.Errorf("%w: expected %d eligible active players; found %d (%d invalid); apply approved dataset", ErrQueueConflict, auction.FantasyPoolSize, total, invalid)
	}
	var count, sequenced, distinct, low, high, bad, stalePrice int
	err = r.db.QueryRow(ctx, `SELECT count(*),count(rp.auction_sequence),count(DISTINCT rp.auction_sequence),coalesce(min(rp.auction_sequence),0),coalesce(max(rp.auction_sequence),0),count(*) FILTER (WHERE rp.status<>'AVAILABLE' OR NOT p.is_active OR NOT p.is_auction_eligible),count(*) FILTER (WHERE rp.base_price<>p.base_price) FROM room_players rp JOIN players p ON p.id=rp.player_id WHERE rp.room_id=$1`, roomID).Scan(&count, &sequenced, &distinct, &low, &high, &bad, &stalePrice)
	if err != nil {
		return 0, err
	}
	if bad != 0 {
		return 0, fmt.Errorf("%w: room contains non-available or ineligible players; use a new room or review data", ErrQueueConflict)
	}
	if !regenerate && sequenced > 0 {
		if count == total && sequenced == total && distinct == total && low == 1 && high == total && stalePrice == 0 {
			var splitCategories bool
			err = r.db.QueryRow(ctx, `WITH ordered AS (
                SELECT p.auction_category,lag(p.auction_category) OVER (ORDER BY rp.auction_sequence) AS previous_category
                FROM room_players rp JOIN players p ON p.id=rp.player_id WHERE rp.room_id=$1
            ) SELECT EXISTS(SELECT 1 FROM ordered WHERE auction_category IS DISTINCT FROM previous_category GROUP BY auction_category HAVING count(*)>1)`, roomID).Scan(&splitCategories)
			if err != nil {
				return 0, err
			}
			if !splitCategories {
				return total, nil
			}
		}
		return 0, fmt.Errorf("%w: prepared queue is stale; regenerate before starting", ErrQueueConflict)
	}
	tag, err := r.db.Exec(ctx, `WITH ordered AS (
 SELECT id,base_price,row_number() OVER (ORDER BY array_position($2::text[],auction_category::text), CASE WHEN auction_category = ANY($3::text[]) THEN random() ELSE 0 END, lower(full_name),id)::integer AS seq
 FROM players WHERE is_active AND is_auction_eligible
 ) INSERT INTO room_players(room_id,player_id,base_price,status,auction_sequence)
 SELECT $1,id,base_price,'AVAILABLE',seq FROM ordered WHERE true
 ON CONFLICT (room_id,player_id) DO UPDATE SET base_price=EXCLUDED.base_price,auction_sequence=EXCLUDED.auction_sequence,updated_at=now()`, roomID, opts.CategoryOrder, opts.ShuffleCategories)
	if err != nil {
		return 0, fmt.Errorf("populate queue: %w", err)
	}
	if tag.RowsAffected() != int64(total) {
		return 0, errors.New("queue population count mismatch")
	}
	return total, nil
}
