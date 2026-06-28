package sync

import (
	"testing"

	"aoi-demo/server/internal/entity"
	"aoi-demo/server/internal/protocol"
)

type testWorldView struct {
	tick     uint64
	entities map[entity.EntityID]entity.Entity
	visible  []entity.Entity
	stats    protocol.ServerStats
}

func (w *testWorldView) Entity(id entity.EntityID) entity.Entity {
	return w.entities[id]
}

func (w *testWorldView) EntitiesForPlayer(_ *entity.Player) []entity.Entity {
	return w.visible
}

func (w *testWorldView) AllEntities() []entity.Entity {
	out := make([]entity.Entity, 0, len(w.entities))
	for _, e := range w.entities {
		out = append(out, e)
	}
	return out
}

func (w *testWorldView) TickID() uint64 {
	return w.tick
}

func (w *testWorldView) Stats() protocol.ServerStats {
	return w.stats
}

func TestDeltaStrategySendsOnlyChangedEntities(t *testing.T) {
	player := testPlayer("player_1", 0, 0, 100)
	npc := testNPC("npc_1", 20, 0)
	world := &testWorldView{
		tick: 1,
		entities: map[entity.EntityID]entity.Entity{
			player.ID(): player,
			npc.ID():    npc,
		},
		visible: []entity.Entity{player, npc},
	}

	strategy := NewDeltaStrategy(1)
	first := updatePayloadFrom(t, strategy.BuildMessages(player, world, nil))
	if first.FullSnapshot {
		t.Fatalf("delta update should be partial, got full snapshot")
	}
	if len(first.Entities) != 2 {
		t.Fatalf("expected initial delta baseline to send 2 entities, got %d", len(first.Entities))
	}

	world.tick = 2
	second := updatePayloadFrom(t, strategy.BuildMessages(player, world, nil))
	if len(second.Entities) != 0 {
		t.Fatalf("expected unchanged delta to send no entities, got %d", len(second.Entities))
	}

	world.tick = 3
	npc.SetPosition(entity.Vec2{X: 25, Y: 0})
	third := updatePayloadFrom(t, strategy.BuildMessages(player, world, nil))
	if len(third.Entities) != 1 || third.Entities[0].ID != npc.ID() {
		t.Fatalf("expected changed npc only, got %+v", third.Entities)
	}
}

func TestPriorityStrategySendsFarEntitiesAtLowFrequency(t *testing.T) {
	player := testPlayer("player_1", 0, 0, 100)
	near := testNPC("npc_near", 40, 0)
	far := testNPC("npc_far", 90, 0)
	world := &testWorldView{
		tick: 1,
		entities: map[entity.EntityID]entity.Entity{
			player.ID(): player,
			near.ID():   near,
			far.ID():    far,
		},
		visible: []entity.Entity{player, near, far},
	}

	strategy := NewPriorityStrategy(1, 3, 0.5)
	first := updatePayloadFrom(t, strategy.BuildMessages(player, world, nil))
	if !first.FullSnapshot || len(first.Entities) != 3 {
		t.Fatalf("expected first priority update to be full, got full=%v entities=%d", first.FullSnapshot, len(first.Entities))
	}

	world.tick = 2
	second := updatePayloadFrom(t, strategy.BuildMessages(player, world, nil))
	if hasEntity(second.Entities, far.ID()) {
		t.Fatalf("expected far entity to skip high-frequency tick, got %+v", second.Entities)
	}
	if !hasEntity(second.Entities, near.ID()) {
		t.Fatalf("expected near entity on high-frequency tick, got %+v", second.Entities)
	}

	world.tick = 3
	third := updatePayloadFrom(t, strategy.BuildMessages(player, world, nil))
	if !hasEntity(third.Entities, far.ID()) {
		t.Fatalf("expected far entity on low-frequency tick, got %+v", third.Entities)
	}
}

func updatePayloadFrom(t *testing.T, messages []protocol.Message) protocol.EntityUpdatePayload {
	t.Helper()
	for _, message := range messages {
		if message.Type != protocol.TypeEntityUpdate {
			continue
		}
		payload, ok := message.Payload.(protocol.EntityUpdatePayload)
		if !ok {
			t.Fatalf("entity_update payload has type %T", message.Payload)
		}
		return payload
	}
	t.Fatalf("missing entity_update in %+v", messages)
	return protocol.EntityUpdatePayload{}
}

func testPlayer(id entity.EntityID, x, y, aoiRadius float64) *entity.Player {
	return &entity.Player{
		BaseEntity: entity.BaseEntity{Id: id, Kind: entity.EntityPlayer, Pos: entity.Vec2{X: x, Y: y}, RadiusV: 8},
		AOIRadius:  aoiRadius,
		VisibleSet: map[entity.EntityID]bool{},
	}
}

func testNPC(id entity.EntityID, x, y float64) *entity.NPC {
	return &entity.NPC{
		BaseEntity: entity.BaseEntity{Id: id, Kind: entity.EntityNPC, Pos: entity.Vec2{X: x, Y: y}, RadiusV: 8},
	}
}

func hasEntity(states []protocol.EntityState, id entity.EntityID) bool {
	for _, state := range states {
		if state.ID == id {
			return true
		}
	}
	return false
}
