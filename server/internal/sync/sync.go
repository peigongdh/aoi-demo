package sync

import (
	"aoi-demo/server/internal/aoi"
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

type SnapshotStrategy struct{}

func NewSnapshotStrategy() *SnapshotStrategy {
	return &SnapshotStrategy{}
}

func (s *SnapshotStrategy) Name() string {
	return "snapshot"
}

func (s *SnapshotStrategy) BuildMessages(player *entity.Player, world WorldView, events []aoi.Event) []protocol.Message {
	messages := make([]protocol.Message, 0, len(events)+1)
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

	messages = append(messages, protocol.Message{
		Type: protocol.TypeEntityUpdate,
		Payload: protocol.EntityUpdatePayload{
			Entities:      StatesFromEntities(world.EntitiesForPlayer(player)),
			DebugEntities: StatesFromEntities(world.AllEntities()),
			ServerTick:    world.TickID(),
			Stats:         world.Stats(),
		},
	})
	return messages
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

type PriorityEvaluator interface {
	Priority(observer entity.EntityID, target entity.EntityID, world WorldView) int
}
