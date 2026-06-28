package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"aoi-demo/server/internal/config"
	"aoi-demo/server/internal/entity"
	"aoi-demo/server/internal/protocol"
	"aoi-demo/server/internal/world"
)

type Server struct {
	cfg      config.Config
	world    *world.World
	http     *http.Server
	netem    *Simulator
	nextID   uint64
	mu       sync.RWMutex
	clients  map[string]*Client
	byPlayer map[entity.EntityID]*Client
}

type Client struct {
	ID       string
	PlayerID entity.EntityID
	Conn     *WSConn
}

func NewServer(cfg config.Config, world *world.World) *Server {
	return &Server{
		cfg:      cfg,
		world:    world,
		netem:    NewSimulator(cfg.Network),
		clients:  map[string]*Client{},
		byPlayer: map[entity.EntityID]*Client{},
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/api/aoi", s.handleAOI)
	mux.HandleFunc("/api/sync", s.handleSync)
	mux.HandleFunc("/api/network", s.handleNetwork)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("AOI worldserver. WebSocket endpoint: /ws\n"))
	})

	s.http = &http.Server{
		Addr:              s.cfg.Server.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go s.tickLoop(ctx)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.http.Shutdown(shutdownCtx)
	}()

	return s.http.ListenAndServe()
}

func (s *Server) tickLoop(ctx context.Context) {
	tickRate := s.cfg.Server.TickRate
	if tickRate <= 0 {
		tickRate = 20
	}
	interval := time.Second / time.Duration(tickRate)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	last := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			dt := now.Sub(last)
			last = now
			outbound := s.world.Tick(dt)
			s.dispatch(outbound)
		}
	}
}

func (s *Server) dispatch(outbound map[entity.EntityID][]protocol.Message) {
	deliveries := []struct {
		client   *Client
		messages []protocol.Message
	}{}

	s.mu.RLock()
	for playerID, messages := range outbound {
		client := s.byPlayer[playerID]
		if client == nil {
			continue
		}
		deliveries = append(deliveries, struct {
			client   *Client
			messages []protocol.Message
		}{client: client, messages: messages})
	}
	s.mu.RUnlock()

	for _, delivery := range deliveries {
		s.sendMessages(delivery.client, delivery.messages)
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgradeWebSocket(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	client := &Client{
		ID:   fmt.Sprintf("%06d", atomic.AddUint64(&s.nextID, 1)),
		Conn: conn,
	}
	s.register(client)
	defer s.unregister(client)

	for {
		data, err := conn.ReadText()
		if err != nil {
			if err != io.EOF {
				fmt.Printf("read from %s failed: %v\n", client.ID, err)
			}
			return
		}
		if err := s.handleMessage(client, data); err != nil {
			fmt.Printf("handle message from %s failed: %v\n", client.ID, err)
		}
	}
}

func (s *Server) register(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[client.ID] = client
}

func (s *Server) unregister(client *Client) {
	_ = client.Conn.Close()

	s.mu.Lock()
	delete(s.clients, client.ID)
	if client.PlayerID != "" {
		delete(s.byPlayer, client.PlayerID)
	}
	playerID := client.PlayerID
	client.PlayerID = ""
	s.mu.Unlock()

	if playerID != "" {
		s.world.RemovePlayer(playerID)
	}
}

func (s *Server) handleMessage(client *Client, data []byte) error {
	var message protocol.IncomingMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return err
	}

	switch message.Type {
	case protocol.TypeJoinWorld:
		return s.handleJoin(client, message)
	case protocol.TypeInput:
		if client.PlayerID == "" {
			return nil
		}
		var payload protocol.InputPayload
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return err
		}
		s.world.SetInput(client.PlayerID, message.Seq, payload)
	case protocol.TypePing:
		var payload protocol.PingPayload
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return err
		}
		s.sendMessage(client, protocol.Message{
			Type: protocol.TypePong,
			Payload: protocol.PongPayload{
				ClientTime: payload.ClientTime,
				ServerTime: time.Now().UnixMilli(),
			},
		})
	}
	return nil
}

func (s *Server) handleJoin(client *Client, message protocol.IncomingMessage) error {
	if client.PlayerID != "" {
		return nil
	}

	var payload protocol.JoinWorldPayload
	if len(message.Payload) > 0 {
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return err
		}
	}

	player, welcome, err := s.world.AddPlayer(client.ID, payload.Name)
	if err != nil {
		return err
	}

	s.mu.Lock()
	client.PlayerID = player.ID()
	s.byPlayer[player.ID()] = client
	s.mu.Unlock()

	s.sendMessage(client, protocol.Message{
		Type:    protocol.TypeWelcome,
		Payload: welcome,
	})
	return nil
}

func (s *Server) sendMessages(client *Client, messages []protocol.Message) {
	if len(messages) == 0 {
		return
	}

	delay := s.netem.Delay()
	send := func() {
		for _, message := range messages {
			if s.netem.ShouldDrop(message.Type) {
				continue
			}
			if err := client.Conn.WriteJSON(message); err != nil {
				fmt.Printf("send to %s failed: %v\n", client.ID, err)
				s.unregister(client)
				return
			}
		}
	}
	if delay <= 0 {
		send()
		return
	}
	time.AfterFunc(delay, send)
}

func (s *Server) sendMessage(client *Client, message protocol.Message) {
	plan := s.netem.Plan(message.Type)
	if plan.Drop {
		return
	}
	send := func() {
		if err := client.Conn.WriteJSON(message); err != nil {
			fmt.Printf("send to %s failed: %v\n", client.ID, err)
			s.unregister(client)
		}
	}
	if plan.Delay <= 0 {
		send()
		return
	}
	time.AfterFunc(plan.Delay, send)
}

func (s *Server) handleAOI(w http.ResponseWriter, r *http.Request) {
	setAPIHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeAOIResponse(w, s.world.AOIName())
	case http.MethodPost:
		var payload struct {
			Type string `json:"type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.world.SetAOIType(payload.Type); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeAOIResponse(w, s.world.AOIName())
	default:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	setAPIHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeSyncResponse(w, s.world.SyncName(), s.cfg.Sync.SnapshotRate)
	case http.MethodPost:
		var payload struct {
			Type string `json:"type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.world.SetSyncType(payload.Type); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeSyncResponse(w, s.world.SyncName(), s.cfg.Sync.SnapshotRate)
	default:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleNetwork(w http.ResponseWriter, r *http.Request) {
	setAPIHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeNetworkResponse(w, s.netem.Config())
	case http.MethodPost:
		var payload config.NetworkConfig
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeNetworkResponse(w, s.netem.Update(payload))
	default:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func setAPIHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func writeAOIResponse(w http.ResponseWriter, current string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Type       string   `json:"type"`
		Algorithms []string `json:"algorithms"`
	}{
		Type:       current,
		Algorithms: []string{"bruteforce", "grid", "tower"},
	})
}

func writeSyncResponse(w http.ResponseWriter, current string, snapshotRate int) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Type         string   `json:"type"`
		Strategies   []string `json:"strategies"`
		SnapshotRate int      `json:"snapshot_rate"`
	}{
		Type:         current,
		Strategies:   []string{"snapshot", "delta", "interpolation", "priority"},
		SnapshotRate: snapshotRate,
	})
}

func writeNetworkResponse(w http.ResponseWriter, cfg config.NetworkConfig) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		LatencyMS  int     `json:"latency_ms"`
		JitterMS   int     `json:"jitter_ms"`
		PacketLoss float64 `json:"packet_loss"`
	}{
		LatencyMS:  cfg.LatencyMS,
		JitterMS:   cfg.JitterMS,
		PacketLoss: cfg.PacketLoss,
	})
}
