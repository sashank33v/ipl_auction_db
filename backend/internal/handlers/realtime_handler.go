package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

const socketWriteTimeout = 5 * time.Second

type liveConnection struct{ cancel context.CancelFunc }
type RealtimeHandler struct {
	ctx         context.Context
	repo        *repository.RealtimeRepository
	sessions    *services.SessionService
	bids        *BidHandler
	auctions    *services.AuctionService
	upgrader    websocket.Upgrader
	mu          sync.Mutex
	connections map[string]*liveConnection
	wg          sync.WaitGroup
	closed      bool
}

func NewRealtimeHandler(ctx context.Context, repo *repository.RealtimeRepository, sessions *services.SessionService, bids *BidHandler, auctions *services.AuctionService, origin string) *RealtimeHandler {
	return &RealtimeHandler{ctx: ctx, repo: repo, sessions: sessions, bids: bids, auctions: auctions, connections: make(map[string]*liveConnection), upgrader: websocket.Upgrader{HandshakeTimeout: 5 * time.Second, CheckOrigin: func(r *http.Request) bool { return allowedSocketOrigin(r.Header.Values("Origin"), origin) }}}
}
func allowedSocketOrigin(values []string, expected string) bool {
	return len(values) == 0 || (len(values) == 1 && expected != "" && values[0] == expected)
}
func (h *RealtimeHandler) Close() {
	h.mu.Lock()
	h.closed = true
	for _, c := range h.connections {
		c.cancel()
	}
	h.mu.Unlock()
	h.wg.Wait()
}
func (h *RealtimeHandler) register(key string, c *liveConnection) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || h.ctx.Err() != nil {
		return false
	}
	if old := h.connections[key]; old != nil {
		old.cancel()
	}
	h.connections[key] = c
	h.wg.Add(1)
	return true
}
func (h *RealtimeHandler) unregister(key string, c *liveConnection) {
	h.mu.Lock()
	if h.connections[key] == c {
		delete(h.connections, key)
	}
	h.mu.Unlock()
	h.wg.Done()
}

type clientMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}
type socketEnvelope struct {
	Type      string    `json:"type"`
	RoomID    string    `json:"room_id"`
	AuctionID string    `json:"auction_id"`
	EventID   string    `json:"event_id,omitempty"`
	Sequence  *int64    `json:"sequence,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload"`
}

func strictJSON(data []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("expected one JSON object")
	}
	return nil
}

func (h *RealtimeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserIDFromContext(r.Context())
	if !ok || user == "" {
		http.Error(w, "Authentication required", 401)
		return
	}
	cookie, err := r.Cookie("ipl_session")
	if err != nil || cookie.Value == "" {
		http.Error(w, "Authentication required", 401)
		return
	}
	if !h.upgrader.CheckOrigin(r) {
		http.Error(w, "Invalid origin", 403)
		return
	}
	hash := sha256.Sum256([]byte(cookie.Value))
	tokenHash := hex.EncodeToString(hash[:])
	auctionID := chi.URLParam(r, "auctionID")
	if !bidUUID.MatchString(auctionID) {
		http.Error(w, "Invalid auction ID", 400)
		return
	}
	ctx, cancel := context.WithCancel(h.ctx)
	defer cancel()
	auth := func() bool {
		check, stop := context.WithTimeout(ctx, 2*time.Second)
		defer stop()
		s, e := h.sessions.GetSessionByTokenHash(check, tokenHash)
		return e == nil && s != nil && s.UserID == user
	}
	if !auth() {
		http.Error(w, "Invalid or expired session", 401)
		return
	}
	query, stop := context.WithTimeout(ctx, 3*time.Second)
	snapshot, _, err := h.repo.Pull(query, auctionID, user, nil, true)
	stop()
	if errors.Is(err, repository.ErrRealtimeForbidden) {
		http.Error(w, "Auction not found or membership required", 403)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load auction", 500)
		return
	}
	roomID := snapshot.Auction.RoomID
	auctionID = snapshot.Auction.ID
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	key := roomID + ":" + user
	c := &liveConnection{cancel: cancel}
	if !h.register(key, c) {
		return
	}
	defer h.unregister(key, c)
	closeOnCancel := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer closeOnCancel()
	conn.SetReadLimit(8 << 10)
	_ = conn.SetReadDeadline(time.Now().Add(40 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(40 * time.Second)) })
	incoming := make(chan clientMessage, 16)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		defer cancel()
		for {
			kind, raw, e := conn.ReadMessage()
			if e != nil {
				return
			}
			if kind != websocket.TextMessage {
				return
			}
			var message clientMessage
			if strictJSON(raw, &message) != nil {
				return
			}
			select {
			case incoming <- message:
			case <-ctx.Done():
				return
			default:
				return
			}
		}
	}()
	defer func() { cancel(); _ = conn.Close(); <-readerDone }()
	send := func(kind string, payload any, seq *int64, eventID string, stamp time.Time) error {
		if err := conn.SetWriteDeadline(time.Now().Add(socketWriteTimeout)); err != nil {
			return err
		}
		return conn.WriteJSON(socketEnvelope{Type: kind, RoomID: roomID, AuctionID: auctionID, Sequence: seq, EventID: eventID, Timestamp: stamp, Payload: payload})
	}
	cursor := snapshot.Sequence
	if send("AUCTION_STATE", snapshot, &cursor, "", snapshot.ServerTime) != nil {
		return
	}
	poll := time.NewTicker(500 * time.Millisecond)
	defer poll.Stop()
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	lastRefresh := time.Now()
	pull := func(force bool, reset bool) bool {
		if !auth() {
			return false
		}
		query, stop := context.WithTimeout(ctx, 3*time.Second)
		defer stop()
		var after *int64 = &cursor
		if reset {
			after = nil
		}
		s, events, e := h.repo.Pull(query, auctionID, user, after, force)
		if e != nil {
			return false
		}
		if s == nil {
			return true
		}
		for _, event := range events {
			kind := event.EventType
			if kind == "PLAYER_NOMINATED" {
				kind = "PLAYER_REVEALED"
			}
			seq := event.SequenceNumber
			if send(kind, event.Payload, &seq, event.ID, event.CreatedAt) != nil {
				return false
			}
		}
		cursor = s.Sequence
		lastRefresh = time.Now()
		return send("AUCTION_STATE", s, &cursor, "", s.ServerTime) == nil
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ping.C:
			if conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(socketWriteTimeout)) != nil {
				return
			}
		case <-poll.C:
			if !pull(time.Since(lastRefresh) >= 2*time.Second, false) {
				return
			}
		case message := <-incoming:
			if !auth() {
				return
			}
			switch message.Type {
			case "REQUEST_SNAPSHOT":
				if !pull(true, true) {
					return
				}
			case "HOST_PAUSE", "HOST_RESUME":
				// Controls have no client-supplied identity, state or timer.
				if len(message.Payload) != 0 {
					var empty struct{}
					if strictJSON(message.Payload, &empty) != nil {
						if send("ERROR", map[string]any{"status": 400, "message": "Control payload must be empty"}, nil, "", time.Now().UTC()) != nil {
							return
						}
						continue
					}
				}
				cmdCtx, stop := context.WithTimeout(ctx, 3*time.Second)
				state, e := h.auctions.SetPaused(cmdCtx, auctionID, user, message.Type == "HOST_PAUSE")
				stop()
				if e != nil {
					status, detail := controlError(e)
					if send("ERROR", map[string]any{"command": message.Type, "status": status, "message": detail}, nil, "", time.Now().UTC()) != nil {
						return
					}
					continue
				}
				if send("CONTROL_RESULT", map[string]any{"command": message.Type, "accepted": true, "auction": state}, nil, "", time.Now().UTC()) != nil {
					return
				}
				if !pull(false, false) {
					return
				}
			case "BID_PLACE":
				var req PlaceBidRequest
				if strictJSON(message.Payload, &req) != nil {
					if send("BID_REJECTED", map[string]any{"message": "Invalid bid payload"}, nil, "", time.Now().UTC()) != nil {
						return
					}
					continue
				}
				cmdCtx, stop := context.WithTimeout(ctx, 3*time.Second)
				bid, status, e := h.bids.placeAuthenticatedBid(cmdCtx, user, auctionID, req)
				stop()
				if e != nil {
					if send("BID_REJECTED", map[string]any{"request_id": req.RequestID, "status": status, "message": e.Error()}, nil, "", time.Now().UTC()) != nil {
						return
					}
					continue
				}
				if send("BID_RESULT", map[string]any{"request_id": req.RequestID, "accepted": true, "bid": bid}, nil, "", time.Now().UTC()) != nil {
					return
				}
				if !pull(false, false) {
					return
				}
			default:
				if send("ERROR", map[string]string{"message": "Unsupported event type"}, nil, "", time.Now().UTC()) != nil {
					return
				}
			}
		}
	}
}
