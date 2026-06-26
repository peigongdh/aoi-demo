package aoi

import (
	"sort"

	"aoi-demo/server/internal/entity"
)

type EventType string

const (
	EventEnter  EventType = "enter"
	EventLeave  EventType = "leave"
	EventUpdate EventType = "update"
)

type Event struct {
	ObserverID entity.EntityID `json:"observer_id"`
	TargetID   entity.EntityID `json:"target_id"`
	Type       EventType       `json:"type"`
}

type WorldState interface {
	Entities() []entity.Entity
	Players() []*entity.Player
}

type Manager interface {
	Name() string
	Init(world WorldState) error
	AddEntity(entity entity.Entity) error
	RemoveEntity(entityID entity.EntityID) error
	MoveEntity(entityID entity.EntityID, oldPos entity.Vec2, newPos entity.Vec2) error
	Query(entityID entity.EntityID, radius float64) ([]entity.EntityID, error)
	Update(world WorldState) ([]Event, error)
}

type baseManager struct {
	entities map[entity.EntityID]entity.Entity
}

func newBaseManager() baseManager {
	return baseManager{entities: map[entity.EntityID]entity.Entity{}}
}

func (b *baseManager) Init(world WorldState) error {
	for _, entity := range world.Entities() {
		b.entities[entity.ID()] = entity
	}
	return nil
}

func (b *baseManager) AddEntity(entity entity.Entity) error {
	b.entities[entity.ID()] = entity
	return nil
}

func (b *baseManager) RemoveEntity(entityID entity.EntityID) error {
	delete(b.entities, entityID)
	return nil
}

func (b *baseManager) MoveEntity(entityID entity.EntityID, _ entity.Vec2, _ entity.Vec2) error {
	if _, ok := b.entities[entityID]; !ok {
		return nil
	}
	return nil
}

func updateByQuery(manager Manager, world WorldState) ([]Event, error) {
	events := []Event{}
	for _, player := range world.Players() {
		ids, err := manager.Query(player.ID(), player.AOIRadius)
		if err != nil {
			return nil, err
		}

		next := map[entity.EntityID]bool{}
		for _, id := range ids {
			next[id] = true
		}

		events = append(events, diffVisibleSet(player.ID(), player.VisibleSet, next)...)
		player.VisibleSet = next
	}
	return events, nil
}

func diffVisibleSet(observerID entity.EntityID, oldSet, newSet map[entity.EntityID]bool) []Event {
	events := []Event{}
	for id := range newSet {
		if !oldSet[id] {
			events = append(events, Event{ObserverID: observerID, TargetID: id, Type: EventEnter})
		}
	}
	for id := range oldSet {
		if !newSet[id] {
			events = append(events, Event{ObserverID: observerID, TargetID: id, Type: EventLeave})
		}
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Type == events[j].Type {
			return events[i].TargetID < events[j].TargetID
		}
		return events[i].Type < events[j].Type
	})
	return events
}

func sortedIDs(set map[entity.EntityID]bool) []entity.EntityID {
	ids := make([]entity.EntityID, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
