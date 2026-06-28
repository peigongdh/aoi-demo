package network

import (
	"math/rand"
	"sync"
	"time"

	"aoi-demo/server/internal/config"
	"aoi-demo/server/internal/protocol"
)

type deliveryPlan struct {
	Delay time.Duration
	Drop  bool
}

type Simulator struct {
	mu  sync.Mutex
	cfg config.NetworkConfig
	rng *rand.Rand
}

func NewSimulator(cfg config.NetworkConfig) *Simulator {
	return &Simulator{
		cfg: normalizeNetworkConfig(cfg),
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *Simulator) Config() config.NetworkConfig {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.cfg
}

func (s *Simulator) Update(cfg config.NetworkConfig) config.NetworkConfig {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cfg = normalizeNetworkConfig(cfg)
	return s.cfg
}

func (s *Simulator) Plan(messageType string) deliveryPlan {
	delay := s.Delay()
	return deliveryPlan{
		Delay: delay,
		Drop:  s.ShouldDrop(messageType),
	}
}

func (s *Simulator) Delay() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	delay := time.Duration(s.cfg.LatencyMS) * time.Millisecond
	if s.cfg.JitterMS > 0 {
		jitter := s.rng.Intn(s.cfg.JitterMS*2+1) - s.cfg.JitterMS
		delay += time.Duration(jitter) * time.Millisecond
		if delay < 0 {
			delay = 0
		}
	}
	return delay
}

func (s *Simulator) ShouldDrop(messageType string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if messageType == protocol.TypeEntityUpdate && s.cfg.PacketLoss > 0 && s.rng.Float64() < s.cfg.PacketLoss {
		return true
	}
	return false
}

func normalizeNetworkConfig(cfg config.NetworkConfig) config.NetworkConfig {
	if cfg.LatencyMS < 0 {
		cfg.LatencyMS = 0
	}
	if cfg.JitterMS < 0 {
		cfg.JitterMS = 0
	}
	if cfg.PacketLoss < 0 {
		cfg.PacketLoss = 0
	}
	if cfg.PacketLoss > 1 {
		cfg.PacketLoss = 1
	}
	return cfg
}
