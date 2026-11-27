package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrRealtimeForbidden = errors.New("auction not found or membership required")

// LiveSnapshot is captured with its event high-water mark in one repeatable
// read transaction, so it cannot combine states from different commits.
type LiveSnapshot struct {
	Auction           *Auction      `json:"auction"`
	RoomStatus        string        `json:"room_status"`
	ServerTime        time.Time     `json:"server_time"`
	Sequence          int64         `json:"sequence"`
	CurrentRoomPlayer *RoomPlayer   `json:"current_room_player"`
	CurrentPlayer     *Player       `json:"current_player"`
	Teams             []Team        `json:"teams"`
	Squads            []SquadPlayer `json:"squads"`
	LastEvent         *AuctionEvent `json:"last_event"`
}

type RealtimeRepository struct{ pool *pgxpool.Pool }

func NewRealtimeRepository(pool *pgxpool.Pool) *RealtimeRepository {
	return &RealtimeRepository{pool: pool}
}

// Pull returns nil if nothing changed and no refresh was requested. A gap or
// oversized replay produces an authoritative snapshot instead of partial replay.
func (r *RealtimeRepository) Pull(ctx context.Context, auctionID, userID string, after *int64, force bool) (*LiveSnapshot, []AuctionEvent, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	a, err := NewAuctionRepository(tx).GetByID(ctx, auctionID)
	if err != nil {
		return nil, nil, err
	}
	if a == nil {
		return nil, nil, ErrRealtimeForbidden
	}
	member, err := NewRoomParticipantRepository(tx).GetByRoomAndUser(ctx, a.RoomID, userID)
	if err != nil {
		return nil, nil, err
	}
	if member == nil {
		return nil, nil, ErrRealtimeForbidden
	}
	s := &LiveSnapshot{Auction: a}
	if err = tx.QueryRow(ctx, `SELECT COALESCE(MAX(sequence_number),0),clock_timestamp() FROM auction_events WHERE auction_id=$1`, a.ID).Scan(&s.Sequence, &s.ServerTime); err != nil {
		return nil, nil, err
	}
	if after != nil && *after == s.Sequence && !force {
		return nil, nil, tx.Commit(ctx)
	}
	if err = tx.QueryRow(ctx, `SELECT status FROM rooms WHERE id=$1`, a.RoomID).Scan(&s.RoomStatus); err != nil {
		return nil, nil, err
	}
	if a.CurrentPlayerID != nil {
		s.CurrentRoomPlayer, err = NewRoomPlayerRepository(tx).GetByID(ctx, *a.CurrentPlayerID)
		if err != nil {
			return nil, nil, err
		}
		if s.CurrentRoomPlayer == nil {
			return nil, nil, errors.New("current room player missing")
		}
		s.CurrentPlayer, err = NewPlayerRepository(tx).GetByID(ctx, s.CurrentRoomPlayer.PlayerID)
		if err != nil {
			return nil, nil, err
		}
		if s.CurrentPlayer == nil {
			return nil, nil, errors.New("current player missing")
		}
	}
	s.Teams, err = NewTeamRepository(tx).GetByRoom(ctx, a.RoomID)
	if err != nil {
		return nil, nil, err
	}
	s.Squads, err = NewSquadRepository(tx).GetByRoom(ctx, a.RoomID)
	if err != nil {
		return nil, nil, err
	}
	eventsRepo := NewAuctionEventRepository(tx)
	if s.Sequence > 0 {
		s.LastEvent, err = eventsRepo.GetBySequence(ctx, a.ID, s.Sequence)
		if err != nil {
			return nil, nil, err
		}
	}
	var events []AuctionEvent
	if after != nil && *after >= 0 && s.Sequence > *after && s.Sequence-*after <= 100 {
		events, err = eventsRepo.GetAfterSequence(ctx, a.ID, *after, 100)
		if err != nil {
			return nil, nil, err
		}
		if !ContiguousEvents(events, *after, s.Sequence) {
			events = nil
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return s, events, nil
}

func ContiguousEvents(events []AuctionEvent, after, last int64) bool {
	if int64(len(events)) != last-after {
		return false
	}
	for i, e := range events {
		if e.SequenceNumber != after+int64(i)+1 {
			return false
		}
	}
	return true
}
