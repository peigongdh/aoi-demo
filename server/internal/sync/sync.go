package sync

import (
	"fmt"
	"strings"
	"time"

	"aoi-demo/server/internal/aoi"
	"aoi-demo/server/internal/config"
	"aoi-demo/server/internal/entity"
	"aoi-demo/server/internal/protocol"
)

type Strategy interface {
	Name() string
	BuildMessages(player *entity.Player, world WorldView, events []aoi.Event) []protocol.Message
}

type WorldView interface {
	Entity(id entity.EntityID) entity.Entity
	EntitiesForPlayer(player *entity.Player) []entity.Entity
	AllEntities() []entity.Entity
	TickID() uint64
	Stats() protocol.ServerStats
}

type PriorityEvaluator interface {
	Priority(observer entity.EntityID, target entity.EntityID, world WorldView) int
}

func New(cfg config.SyncConfig, tickRate int) (Strategy, error) {
	interval := tickInterval(tickRate, cfg.SnapshotRate)
	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "", "snapshot":
		return NewSnapshotStrategy(interval), nil
	case "delta":
		return NewDeltaStrategy(interval), nil
	case "interpolation":
		return NewInterpolationStrategy(interval), nil
	case "priority":
		lowInterval := tickInterval(tickRate, cfg.PriorityLowRate)
		return NewPriorityStrategy(interval, lowInterval, cfg.PriorityNearRatio), nil
	default:
		return nil, fmt.Errorf("unsupported sync type %q", cfg.Type)
	}
}

type timedStrategy struct {
	name         string
	tickInterval uint64
}

func (s timedStrategy) Name() string {
	return s.name
}

func (s timedStrategy) shouldSend(tick uint64) bool {
	return s.tickInterval <= 1 || tick%s.tickInterval == 0
}

type SnapshotStrategy struct {
	timedStrategy
}

func NewSnapshotStrategy(interval ...uint64) *SnapshotStrategy {
	return &SnapshotStrategy{
		timedStrategy: timedStrategy{name: "snapshot", tickInterval: optionalInterval(interval)},
	}
}

func (s *SnapshotStrategy) BuildMessages(player *entity.Player, world WorldView, events []aoi.Event) []protocol.Message {
	messages := eventMessages(world, events)
	if !s.shouldSend(world.TickID()) {
		return messages
	}
	messages = append(messages, protocol.Message{
		Type:    protocol.TypeEntityUpdate,
		Payload: updatePayload(s.Name(), true, StatesFromEntities(world.EntitiesForPlayer(player)), world),
	})
	return messages
}

type InterpolationStrategy struct {
	*SnapshotStrategy
}

func NewInterpolationStrategy(interval uint64) *InterpolationStrategy {
	return &InterpolationStrategy{SnapshotStrategy: &SnapshotStrategy{
		timedStrategy: timedStrategy{name: "interpolation", tickInterval: optionalInterval([]uint64{interval})},
	}}
}

type DeltaStrategy struct {
	timedStrategy
	last map[entity.EntityID]map[entity.EntityID]protocol.EntityState
}

func NewDeltaStrategy(interval uint64) *DeltaStrategy {
	return &DeltaStrategy{
		timedStrategy: timedStrategy{name: "delta", tickInterval: optionalInterval([]uint64{interval})},
		last:          map[entity.EntityID]map[entity.EntityID]protocol.EntityState{},
	}
}

func (s *DeltaStrategy) BuildMessages(player *entity.Player, world WorldView, events []aoi.Event) []protocol.Message {
	last := s.last[player.ID()]
	if last == nil {
		last = map[entity.EntityID]protocol.EntityState{}
		s.last[player.ID()] = last
	}

	messages := eventMessages(world, events)
	applyEventsToBaseline(last, world, events)
	if !s.shouldSend(world.TickID()) {
		return messages
	}

	changed := []protocol.EntityState{}
	for _, e := range world.EntitiesForPlayer(player) {
		state := StateFromEntity(e)
		previous, ok := last[state.ID]
		if !ok || !sameState(previous, state) {
			changed = append(changed, state)
			last[state.ID] = state
		}
	}

	messages = append(messages, protocol.Message{
		Type:    protocol.TypeEntityUpdate,
		Payload: updatePayload(s.Name(), false, changed, world),
	})
	return messages
}

type PriorityStrategy struct {
	timedStrategy
	lowTickInterval uint64
	nearRatio       float64
	initialized     map[entity.EntityID]bool
}

func NewPriorityStrategy(interval, lowInterval uint64, nearRatio float64) *PriorityStrategy {
	if nearRatio <= 0 {
		nearRatio = 0.5
	}
	if nearRatio > 1 {
		nearRatio = 1
	}
	return &PriorityStrategy{
		timedStrategy:   timedStrategy{name: "priority", tickInterval: optionalInterval([]uint64{interval})},
		lowTickInterval: optionalInterval([]uint64{lowInterval}),
		nearRatio:       nearRatio,
		initialized:     map[entity.EntityID]bool{},
	}
}

func (s *PriorityStrategy) BuildMessages(player *entity.Player, world WorldView, events []aoi.Event) []protocol.Message {
	messages := eventMessages(world, events)
	if !s.shouldSend(world.TickID()) {
		return messages
	}

	fullInitial := !s.initialized[player.ID()]
	s.initialized[player.ID()] = true
	lowTick := s.lowTickInterval <= 1 || world.TickID()%s.lowTickInterval == 0
	nearRadius := player.AOIRadius * s.nearRatio
	observerPos := player.Position()
	selected := []protocol.EntityState{}

	for _, e := range world.EntitiesForPlayer(player) {
		if fullInitial || e.ID() == player.ID() || lowTick || entity.Distance(observerPos, e.Position()) <= nearRadius {
			selected = append(selected, StateFromEntity(e))
		}
	}

	messages = append(messages, protocol.Message{
		Type:    protocol.TypeEntityUpdate,
		Payload: updatePayload(s.Name(), fullInitial, selected, world),
	})
	return messages
}

func tickInterval(tickRate, rate int) uint64 {
	if tickRate <= 0 || rate <= 0 || rate >= tickRate {
		return 1
	}
	interval := tickRate / rate
	if interval < 1 {
		return 1
	}
	return uint64(interval)
}

func optionalInterval(interval []uint64) uint64 {
	if len(interval) == 0 || interval[0] == 0 {
		return 1
	}
	return interval[0]
}

func eventMessages(world WorldView, events []aoi.Event) []protocol.Message {
	messages := make([]protocol.Message, 0, len(events))
	for _, event := range events {
		switch event.Type {
		case aoi.EventEnter:
			target := world.Entity(event.TargetID)
			if target == nil {
				continue
			}
			messages = append(messages, protocol.Message{
				Type: protocol.TypeEntityEnter,
				Payload: protocol.EntityEnterPayload{
					Entity: StateFromEntity(target),
				},
			})
		case aoi.EventLeave:
			messages = append(messages, protocol.Message{
				Type: protocol.TypeEntityLeave,
				Payload: protocol.EntityLeavePayload{
					EntityID: event.TargetID,
				},
			})
		}
	}
	return messages
}

func applyEventsToBaseline(last map[entity.EntityID]protocol.EntityState, world WorldView, events []aoi.Event) {
	for _, event := range events {
		switch event.Type {
		case aoi.EventEnter:
			if target := world.Entity(event.TargetID); target != nil {
				last[event.TargetID] = StateFromEntity(target)
			}
		case aoi.EventLeave:
			delete(last, event.TargetID)
		}
	}
}

func updatePayload(syncType string, fullSnapshot bool, states []protocol.EntityState, world WorldView) protocol.EntityUpdatePayload {
	return protocol.EntityUpdatePayload{
		Entities:      states,
		DebugEntities: StatesFromEntities(world.AllEntities()),
		ServerTick:    world.TickID(),
		ServerTime:    time.Now().UnixMilli(),
		SyncType:      syncType,
		FullSnapshot:  fullSnapshot,
		Stats:         world.Stats(),
	}
}

func sameState(a, b protocol.EntityState) bool {
	return a.ID == b.ID &&
		a.Type == b.Type &&
		a.Name == b.Name &&
		a.X == b.X &&
		a.Y == b.Y &&
		a.Radius == b.Radius &&
		a.LastInputSeq == b.LastInputSeq
}

func StateFromEntity(e entity.Entity) protocol.EntityState {
	pos := e.Position()
	state := protocol.EntityState{
		ID:     e.ID(),
		Type:   e.Type(),
		X:      pos.X,
		Y:      pos.Y,
		Radius: e.Radius(),
	}
	if player, ok := e.(*entity.Player); ok {
		state.Name = player.Name
		state.LastInputSeq = player.LastInputSeq
	}
	return state
}

func StatesFromEntities(entities []entity.Entity) []protocol.EntityState {
	states := make([]protocol.EntityState, 0, len(entities))
	for _, e := range entities {
		if e == nil {
			continue
		}
		states = append(states, StateFromEntity(e))
	}
	return states
}
