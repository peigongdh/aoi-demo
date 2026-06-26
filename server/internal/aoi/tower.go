package aoi

import (
	"math"

	"aoi-demo/server/internal/entity"
)

type TowerCoord struct {
	X int
	Y int
}

type TowerCell struct {
	Coord    TowerCoord
	Entities map[entity.EntityID]bool
	Watchers map[entity.EntityID]bool
}

type Tower struct {
	baseManager
	width         float64
	height        float64
	towerSize     float64
	cols          int
	rows          int
	towers        map[TowerCoord]*TowerCell
	positions     map[entity.EntityID]TowerCoord
	subscriptions map[entity.EntityID]map[TowerCoord]bool
}

func NewTower(width, height, towerSize float64) *Tower {
	if towerSize <= 0 {
		towerSize = 200
	}
	cols := int(math.Ceil(width / towerSize))
	rows := int(math.Ceil(height / towerSize))
	if cols <= 0 {
		cols = 1
	}
	if rows <= 0 {
		rows = 1
	}
	return &Tower{
		baseManager:   newBaseManager(),
		width:         width,
		height:        height,
		towerSize:     towerSize,
		cols:          cols,
		rows:          rows,
		towers:        map[TowerCoord]*TowerCell{},
		positions:     map[entity.EntityID]TowerCoord{},
		subscriptions: map[entity.EntityID]map[TowerCoord]bool{},
	}
}

func (a *Tower) Name() string {
	return "tower"
}

func (a *Tower) Init(world WorldState) error {
	for _, entity := range world.Entities() {
		if err := a.AddEntity(entity); err != nil {
			return err
		}
	}
	for _, player := range world.Players() {
		a.setWatcher(player.ID(), a.coordSet(player.Position(), player.AOIRadius))
	}
	return nil
}

func (a *Tower) AddEntity(e entity.Entity) error {
	a.entities[e.ID()] = e
	coord := a.PosToTower(e.Position())
	a.getTower(coord).Entities[e.ID()] = true
	a.positions[e.ID()] = coord
	if player, ok := e.(*entity.Player); ok {
		a.setWatcher(player.ID(), a.coordSet(player.Position(), player.AOIRadius))
	}
	return nil
}

func (a *Tower) RemoveEntity(entityID entity.EntityID) error {
	if coord, ok := a.positions[entityID]; ok {
		if tower := a.towers[coord]; tower != nil {
			delete(tower.Entities, entityID)
			a.pruneTower(coord)
		}
	}
	if subscribed, ok := a.subscriptions[entityID]; ok {
		for coord := range subscribed {
			if tower := a.towers[coord]; tower != nil {
				delete(tower.Watchers, entityID)
				a.pruneTower(coord)
			}
		}
		delete(a.subscriptions, entityID)
	}
	delete(a.positions, entityID)
	delete(a.entities, entityID)
	return nil
}

func (a *Tower) MoveEntity(entityID entity.EntityID, oldPos entity.Vec2, newPos entity.Vec2) error {
	oldCoord, ok := a.positions[entityID]
	if !ok {
		oldCoord = a.PosToTower(oldPos)
	}
	newCoord := a.PosToTower(newPos)
	if oldCoord != newCoord {
		if tower := a.towers[oldCoord]; tower != nil {
			delete(tower.Entities, entityID)
			a.pruneTower(oldCoord)
		}
		a.getTower(newCoord).Entities[entityID] = true
	}
	a.positions[entityID] = newCoord

	if player, ok := a.entities[entityID].(*entity.Player); ok {
		a.setWatcher(player.ID(), a.coordSet(newPos, player.AOIRadius))
	}
	return nil
}

func (a *Tower) Query(observerID entity.EntityID, radius float64) ([]entity.EntityID, error) {
	observer := a.entities[observerID]
	if observer == nil {
		return nil, nil
	}

	coords := a.coordSet(observer.Position(), radius)
	if _, ok := observer.(*entity.Player); ok {
		a.setWatcher(observerID, coords)
	}

	candidates := map[entity.EntityID]bool{}
	for coord := range coords {
		tower := a.towers[coord]
		if tower == nil {
			continue
		}
		for id := range tower.Entities {
			candidates[id] = true
		}
	}

	visible := map[entity.EntityID]bool{}
	for id := range candidates {
		if id == observerID {
			continue
		}
		target := a.entities[id]
		if target == nil {
			continue
		}
		if entity.Distance(observer.Position(), target.Position()) <= radius {
			visible[id] = true
		}
	}
	return sortedIDs(visible), nil
}

func (a *Tower) Update(world WorldState) ([]Event, error) {
	return updateByQuery(a, world)
}

func (a *Tower) PosToTower(pos entity.Vec2) TowerCoord {
	x := int(math.Floor(pos.X / a.towerSize))
	y := int(math.Floor(pos.Y / a.towerSize))
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x >= a.cols {
		x = a.cols - 1
	}
	if y >= a.rows {
		y = a.rows - 1
	}
	return TowerCoord{X: x, Y: y}
}

func (a *Tower) coordSet(pos entity.Vec2, radius float64) map[TowerCoord]bool {
	center := a.PosToTower(pos)
	towerRange := int(math.Ceil(radius / a.towerSize))
	coords := map[TowerCoord]bool{}
	for x := center.X - towerRange; x <= center.X+towerRange; x++ {
		for y := center.Y - towerRange; y <= center.Y+towerRange; y++ {
			coord := TowerCoord{X: x, Y: y}
			if a.inBounds(coord) {
				coords[coord] = true
			}
		}
	}
	return coords
}

func (a *Tower) setWatcher(observerID entity.EntityID, next map[TowerCoord]bool) {
	current := a.subscriptions[observerID]
	for coord := range current {
		if !next[coord] {
			if tower := a.towers[coord]; tower != nil {
				delete(tower.Watchers, observerID)
				a.pruneTower(coord)
			}
		}
	}
	for coord := range next {
		if current == nil || !current[coord] {
			a.getTower(coord).Watchers[observerID] = true
		}
	}
	a.subscriptions[observerID] = next
}

func (a *Tower) getTower(coord TowerCoord) *TowerCell {
	tower := a.towers[coord]
	if tower != nil {
		return tower
	}
	tower = &TowerCell{
		Coord:    coord,
		Entities: map[entity.EntityID]bool{},
		Watchers: map[entity.EntityID]bool{},
	}
	a.towers[coord] = tower
	return tower
}

func (a *Tower) inBounds(coord TowerCoord) bool {
	return coord.X >= 0 && coord.Y >= 0 && coord.X < a.cols && coord.Y < a.rows
}

func (a *Tower) pruneTower(coord TowerCoord) {
	tower := a.towers[coord]
	if tower == nil {
		return
	}
	if len(tower.Entities) == 0 && len(tower.Watchers) == 0 {
		delete(a.towers, coord)
	}
}
