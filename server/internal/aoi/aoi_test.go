package aoi

import (
	"reflect"
	"testing"

	"aoi-demo/server/internal/entity"
)

type testWorld struct {
	entities []entity.Entity
	players  []*entity.Player
}

func (w testWorld) Entities() []entity.Entity {
	return w.entities
}

func (w testWorld) Players() []*entity.Player {
	return w.players
}

func TestSpatialAOIMatchesBruteForce(t *testing.T) {
	player := &entity.Player{
		BaseEntity: entity.BaseEntity{Id: "player_1", Kind: entity.EntityPlayer, Pos: entity.Vec2{X: 100, Y: 100}, RadiusV: 8},
		AOIRadius:  200,
		VisibleSet: map[entity.EntityID]bool{},
	}
	nearNPC := &entity.NPC{BaseEntity: entity.BaseEntity{Id: "npc_near", Kind: entity.EntityNPC, Pos: entity.Vec2{X: 250, Y: 120}, RadiusV: 8}}
	farNPC := &entity.NPC{BaseEntity: entity.BaseEntity{Id: "npc_far", Kind: entity.EntityNPC, Pos: entity.Vec2{X: 450, Y: 450}, RadiusV: 8}}
	edgeNPC := &entity.NPC{BaseEntity: entity.BaseEntity{Id: "npc_edge", Kind: entity.EntityNPC, Pos: entity.Vec2{X: 300, Y: 100}, RadiusV: 8}}
	world := testWorld{
		entities: []entity.Entity{player, nearNPC, farNPC, edgeNPC},
		players:  []*entity.Player{player},
	}

	brute := NewBruteForce()
	if err := brute.Init(world); err != nil {
		t.Fatal(err)
	}
	bruteIDs, err := brute.Query(player.ID(), player.AOIRadius)
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]Manager{
		"grid":  NewGrid(1000, 1000, 100),
		"tower": NewTower(1000, 1000, 100),
	}
	for name, manager := range cases {
		t.Run(name, func(t *testing.T) {
			if err := manager.Init(world); err != nil {
				t.Fatal(err)
			}
			ids, err := manager.Query(player.ID(), player.AOIRadius)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(bruteIDs, ids) {
				t.Fatalf("%s result mismatch: brute=%v got=%v", name, bruteIDs, ids)
			}
		})
	}
}

func TestTowerUpdatesEntityCellsOnMove(t *testing.T) {
	player := &entity.Player{
		BaseEntity: entity.BaseEntity{Id: "player_1", Kind: entity.EntityPlayer, Pos: entity.Vec2{X: 100, Y: 100}, RadiusV: 8},
		AOIRadius:  120,
		VisibleSet: map[entity.EntityID]bool{},
	}
	npc := &entity.NPC{BaseEntity: entity.BaseEntity{Id: "npc_1", Kind: entity.EntityNPC, Pos: entity.Vec2{X: 500, Y: 500}, RadiusV: 8}}
	world := testWorld{
		entities: []entity.Entity{player, npc},
		players:  []*entity.Player{player},
	}

	tower := NewTower(1000, 1000, 100)
	if err := tower.Init(world); err != nil {
		t.Fatal(err)
	}
	ids, err := tower.Query(player.ID(), player.AOIRadius)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected npc outside AOI, got %v", ids)
	}

	oldPos := npc.Position()
	newPos := entity.Vec2{X: 180, Y: 100}
	npc.SetPosition(newPos)
	if err := tower.MoveEntity(npc.ID(), oldPos, newPos); err != nil {
		t.Fatal(err)
	}
	ids, err = tower.Query(player.ID(), player.AOIRadius)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids, []entity.EntityID{"npc_1"}) {
		t.Fatalf("expected moved npc visible, got %v", ids)
	}
}

func TestVisibleSetDiff(t *testing.T) {
	events := diffVisibleSet("player_1", map[entity.EntityID]bool{
		"npc_leave": true,
		"npc_keep":  true,
	}, map[entity.EntityID]bool{
		"npc_enter": true,
		"npc_keep":  true,
	})

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	seen := map[EventType]entity.EntityID{}
	for _, event := range events {
		seen[event.Type] = event.TargetID
	}
	if seen[EventEnter] != "npc_enter" {
		t.Fatalf("missing enter event: %v", events)
	}
	if seen[EventLeave] != "npc_leave" {
		t.Fatalf("missing leave event: %v", events)
	}
}
