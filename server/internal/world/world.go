package world

import (
	"fmt"
	"math/rand"
	"strings"
	stdsync "sync"
	"time"

	"aoi-demo/server/internal/aoi"
	"aoi-demo/server/internal/config"
	"aoi-demo/server/internal/entity"
	"aoi-demo/server/internal/protocol"
	syncstrategy "aoi-demo/server/internal/sync"
)

type WorldMap struct {
	Width  float64
	Height float64
}

type World struct {
	mu       stdsync.Mutex
	cfg      config.Config
	worldMap WorldMap
	entities *entity.Manager
	aoi      aoi.Manager
	sync     syncstrategy.Strategy
	inputs   map[entity.EntityID]protocol.InputPayload
	tick     uint64
	stats    protocol.ServerStats
	rng      *rand.Rand
}

func New(cfg config.Config) (*World, error) {
	entities := entity.NewManager()
	aoiManager, err := makeAOI(cfg)
	if err != nil {
		return nil, err
	}
	syncStrategy, err := makeSync(cfg)
	if err != nil {
		return nil, err
	}

	w := &World{
		cfg: cfg,
		worldMap: WorldMap{
			Width:  cfg.World.Width,
			Height: cfg.World.Height,
		},
		entities: entities,
		aoi:      aoiManager,
		sync:     syncStrategy,
		inputs:   map[entity.EntityID]protocol.InputPayload{},
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	if err := w.aoi.Init(w); err != nil {
		return nil, err
	}
	w.spawnNPCs(cfg.World.NPCCount)
	return w, nil
}

func makeAOI(cfg config.Config) (aoi.Manager, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.AOI.Type)) {
	case "bruteforce":
		return aoi.NewBruteForce(), nil
	case "grid":
		return aoi.NewGrid(cfg.World.Width, cfg.World.Height, cfg.AOI.GridSize), nil
	case "tower":
		return aoi.NewTower(cfg.World.Width, cfg.World.Height, cfg.AOI.GridSize), nil
	default:
		return nil, fmt.Errorf("unsupported aoi type %q", cfg.AOI.Type)
	}
}

func makeSync(cfg config.Config) (syncstrategy.Strategy, error) {
	return syncstrategy.New(cfg.Sync, cfg.Server.TickRate)
}

func (w *World) AddPlayer(connID, name string) (*entity.Player, protocol.WelcomePayload, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if name == "" {
		name = connID
	}
	id := entity.EntityID(fmt.Sprintf("player_%s", connID))
	player := &entity.Player{
		BaseEntity: entity.BaseEntity{
			Id:      id,
			Kind:    entity.EntityPlayer,
			Pos:     w.randomSpawn(),
			RadiusV: 8,
		},
		ConnID:     connID,
		Name:       name,
		AOIRadius:  w.cfg.Player.AOIRadius,
		VisibleSet: map[entity.EntityID]bool{},
	}

	w.entities.Add(player)
	if err := w.aoi.AddEntity(player); err != nil {
		w.entities.Remove(player.ID())
		return nil, protocol.WelcomePayload{}, err
	}

	welcome := protocol.WelcomePayload{
		PlayerID:    player.ID(),
		MapWidth:    w.worldMap.Width,
		MapHeight:   w.worldMap.Height,
		AOIType:     w.aoi.Name(),
		AOIRadius:   player.AOIRadius,
		GridSize:    w.cfg.AOI.GridSize,
		SyncType:    w.sync.Name(),
		PlayerSpeed: w.cfg.Player.Speed,
		ServerTick:  w.tick,
		Entity:      syncstrategy.StateFromEntity(player),
	}
	return player, welcome, nil
}

func (w *World) RemovePlayer(id entity.EntityID) {
	w.mu.Lock()
	defer w.mu.Unlock()

	delete(w.inputs, id)
	_ = w.aoi.RemoveEntity(id)
	w.entities.Remove(id)
	for _, player := range w.entities.Players() {
		delete(player.VisibleSet, id)
	}
}

func (w *World) SetInput(id entity.EntityID, seq uint64, input protocol.InputPayload) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if player, ok := w.entities.Get(id).(*entity.Player); ok {
		player.LastInputSeq = seq
		w.inputs[id] = input
	}
}

func (w *World) Tick(dt time.Duration) map[entity.EntityID][]protocol.Message {
	w.mu.Lock()
	defer w.mu.Unlock()

	tickStart := time.Now()
	w.tick++
	w.updatePlayers(dt)
	w.updateNPCs(dt)

	aoiStart := time.Now()
	events, err := w.aoi.Update(w)
	aoiDuration := time.Since(aoiStart)
	if err != nil {
		fmt.Printf("aoi update failed: %v\n", err)
		events = nil
	}
	eventsByPlayer := map[entity.EntityID][]aoi.Event{}
	for _, event := range events {
		eventsByPlayer[event.ObserverID] = append(eventsByPlayer[event.ObserverID], event)
	}

	players := w.entities.Players()
	npcs := w.entities.NPCs()
	entities := w.entities.All()
	w.stats = protocol.ServerStats{
		PlayerCount:    len(players),
		NPCCount:       len(npcs),
		EntityCount:    len(entities),
		SyncType:       w.sync.Name(),
		AOIQueryMS:     durationMS(aoiDuration),
		TickDurationMS: durationMS(time.Since(tickStart)),
		AOIEvents:      len(events),
	}

	out := map[entity.EntityID][]protocol.Message{}
	messagesOut := 0
	for _, player := range players {
		messages := w.sync.BuildMessages(player, w, eventsByPlayer[player.ID()])
		out[player.ID()] = messages
		messagesOut += len(messages)
	}
	w.stats.MessagesOut = messagesOut
	w.stats.TickDurationMS = durationMS(time.Since(tickStart))
	patchStats(out, w.stats)
	return out
}

func (w *World) updatePlayers(dt time.Duration) {
	seconds := dt.Seconds()
	for _, player := range w.entities.Players() {
		input := w.inputs[player.ID()]
		dir := entity.Vec2{}
		if input.Up {
			dir.Y -= 1
		}
		if input.Down {
			dir.Y += 1
		}
		if input.Left {
			dir.X -= 1
		}
		if input.Right {
			dir.X += 1
		}
		velocity := dir.Normalize().Scale(w.cfg.Player.Speed)
		oldPos := player.Position()
		newPos := w.clamp(oldPos.Add(velocity.Scale(seconds)))
		player.SetVelocity(velocity)
		player.SetPosition(newPos)
		if oldPos != newPos {
			_ = w.aoi.MoveEntity(player.ID(), oldPos, newPos)
		}
	}
}

func (w *World) updateNPCs(dt time.Duration) {
	seconds := dt.Seconds()
	elapsedMS := int(dt / time.Millisecond)
	for _, npc := range w.entities.NPCs() {
		if npc.Behavior != entity.NPCWander {
			continue
		}

		npc.NextTurnInMS -= elapsedMS
		if npc.NextTurnInMS <= 0 {
			npc.SetVelocity(w.randomDirection().Scale(w.cfg.NPC.Speed))
			npc.NextTurnInMS = 800 + w.rng.Intn(2200)
		}

		oldPos := npc.Position()
		newPos := w.clamp(oldPos.Add(npc.Velocity().Scale(seconds)))
		if newPos.X == 0 || newPos.X == w.worldMap.Width || newPos.Y == 0 || newPos.Y == w.worldMap.Height {
			npc.SetVelocity(w.randomDirection().Scale(w.cfg.NPC.Speed))
		}
		npc.SetPosition(newPos)
		if oldPos != newPos {
			_ = w.aoi.MoveEntity(npc.ID(), oldPos, newPos)
		}
	}
}

func (w *World) spawnNPCs(count int) {
	for i := 0; i < count; i++ {
		behavior := entity.NPCStatic
		velocity := entity.Vec2{}
		if w.cfg.NPC.RandomWalk && i%4 != 0 {
			behavior = entity.NPCWander
			velocity = w.randomDirection().Scale(w.cfg.NPC.Speed)
		}
		npc := &entity.NPC{
			BaseEntity: entity.BaseEntity{
				Id:      entity.EntityID(fmt.Sprintf("npc_%03d", i+1)),
				Kind:    entity.EntityNPC,
				Pos:     w.randomSpawn(),
				Vel:     velocity,
				RadiusV: 8,
			},
			Behavior:     behavior,
			NextTurnInMS: 500 + w.rng.Intn(2500),
		}
		w.entities.Add(npc)
		_ = w.aoi.AddEntity(npc)
	}
}

func (w *World) randomSpawn() entity.Vec2 {
	margin := 40.0
	return entity.Vec2{
		X: margin + w.rng.Float64()*(w.worldMap.Width-margin*2),
		Y: margin + w.rng.Float64()*(w.worldMap.Height-margin*2),
	}
}

func (w *World) randomDirection() entity.Vec2 {
	return entity.Vec2{
		X: w.rng.Float64()*2 - 1,
		Y: w.rng.Float64()*2 - 1,
	}.Normalize()
}

func (w *World) clamp(pos entity.Vec2) entity.Vec2 {
	if pos.X < 0 {
		pos.X = 0
	}
	if pos.Y < 0 {
		pos.Y = 0
	}
	if pos.X > w.worldMap.Width {
		pos.X = w.worldMap.Width
	}
	if pos.Y > w.worldMap.Height {
		pos.Y = w.worldMap.Height
	}
	return pos
}

func (w *World) AOIName() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.aoi.Name()
}

func (w *World) SetAOIType(aoiType string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	nextType := strings.ToLower(strings.TrimSpace(aoiType))
	if nextType == "" {
		return fmt.Errorf("aoi type is required")
	}
	if nextType == w.aoi.Name() {
		return nil
	}

	nextCfg := w.cfg
	nextCfg.AOI.Type = nextType
	nextAOI, err := makeAOI(nextCfg)
	if err != nil {
		return err
	}
	if err := nextAOI.Init(w); err != nil {
		return err
	}
	w.aoi = nextAOI
	w.cfg = nextCfg
	for _, player := range w.entities.Players() {
		player.VisibleSet = map[entity.EntityID]bool{}
	}
	return nil
}

func (w *World) SyncName() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.sync.Name()
}

func (w *World) SetSyncType(syncType string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	nextType := strings.ToLower(strings.TrimSpace(syncType))
	if nextType == "" {
		return fmt.Errorf("sync type is required")
	}
	if nextType == w.sync.Name() {
		return nil
	}

	nextCfg := w.cfg
	nextCfg.Sync.Type = nextType
	nextSync, err := makeSync(nextCfg)
	if err != nil {
		return err
	}
	w.sync = nextSync
	w.cfg = nextCfg
	w.stats.SyncType = nextSync.Name()
	return nil
}

func (w *World) Stats() protocol.ServerStats {
	return w.stats
}

func (w *World) Entities() []entity.Entity {
	return w.entities.All()
}

func (w *World) Players() []*entity.Player {
	return w.entities.Players()
}

func (w *World) Entity(id entity.EntityID) entity.Entity {
	return w.entities.Get(id)
}

func (w *World) EntitiesForPlayer(player *entity.Player) []entity.Entity {
	out := []entity.Entity{}
	if self := w.entities.Get(player.ID()); self != nil {
		out = append(out, self)
	}
	for id := range player.VisibleSet {
		if entity := w.entities.Get(id); entity != nil {
			out = append(out, entity)
		}
	}
	return out
}

func (w *World) AllEntities() []entity.Entity {
	return w.entities.All()
}

func (w *World) TickID() uint64 {
	return w.tick
}

func durationMS(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

func patchStats(out map[entity.EntityID][]protocol.Message, stats protocol.ServerStats) {
	for playerID, messages := range out {
		for i, message := range messages {
			payload, ok := message.Payload.(protocol.EntityUpdatePayload)
			if !ok {
				continue
			}
			payload.Stats = stats
			message.Payload = payload
			messages[i] = message
		}
		out[playerID] = messages
	}
}
