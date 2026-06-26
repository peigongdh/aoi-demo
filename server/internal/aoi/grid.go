package aoi

import (
	"math"

	"aoi-demo/server/internal/entity"
)

type GridCoord struct {
	X int
	Y int
}

type Grid struct {
	baseManager
	width     float64
	height    float64
	gridSize  float64
	cols      int
	rows      int
	cells     map[GridCoord]map[entity.EntityID]bool
	positions map[entity.EntityID]GridCoord
}

func NewGrid(width, height, gridSize float64) *Grid {
	if gridSize <= 0 {
		gridSize = 200
	}
	cols := int(math.Ceil(width / gridSize))
	rows := int(math.Ceil(height / gridSize))
	if cols <= 0 {
		cols = 1
	}
	if rows <= 0 {
		rows = 1
	}
	return &Grid{
		baseManager: newBaseManager(),
		width:       width,
		height:      height,
		gridSize:    gridSize,
		cols:        cols,
		rows:        rows,
		cells:       map[GridCoord]map[entity.EntityID]bool{},
		positions:   map[entity.EntityID]GridCoord{},
	}
}

func (a *Grid) Name() string {
	return "grid"
}

func (a *Grid) Init(world WorldState) error {
	for _, entity := range world.Entities() {
		if err := a.AddEntity(entity); err != nil {
			return err
		}
	}
	return nil
}

func (a *Grid) AddEntity(entity entity.Entity) error {
	a.entities[entity.ID()] = entity
	coord := a.PosToGrid(entity.Position())
	a.addToCell(coord, entity.ID())
	a.positions[entity.ID()] = coord
	return nil
}

func (a *Grid) RemoveEntity(entityID entity.EntityID) error {
	if coord, ok := a.positions[entityID]; ok {
		a.removeFromCell(coord, entityID)
	}
	delete(a.positions, entityID)
	delete(a.entities, entityID)
	return nil
}

func (a *Grid) MoveEntity(entityID entity.EntityID, oldPos entity.Vec2, newPos entity.Vec2) error {
	oldCoord := a.PosToGrid(oldPos)
	newCoord := a.PosToGrid(newPos)
	if oldCoord == newCoord {
		a.positions[entityID] = newCoord
		return nil
	}
	a.removeFromCell(oldCoord, entityID)
	a.addToCell(newCoord, entityID)
	a.positions[entityID] = newCoord
	return nil
}

func (a *Grid) Query(observerID entity.EntityID, radius float64) ([]entity.EntityID, error) {
	observer := a.entities[observerID]
	if observer == nil {
		return nil, nil
	}

	center := a.PosToGrid(observer.Position())
	gridRange := int(math.Ceil(radius / a.gridSize))
	candidates := map[entity.EntityID]bool{}
	for gx := center.X - gridRange; gx <= center.X+gridRange; gx++ {
		for gy := center.Y - gridRange; gy <= center.Y+gridRange; gy++ {
			coord := GridCoord{X: gx, Y: gy}
			cell := a.cells[coord]
			if cell == nil {
				continue
			}
			for id := range cell {
				candidates[id] = true
			}
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

func (a *Grid) Update(world WorldState) ([]Event, error) {
	return updateByQuery(a, world)
}

func (a *Grid) PosToGrid(pos entity.Vec2) GridCoord {
	x := int(math.Floor(pos.X / a.gridSize))
	y := int(math.Floor(pos.Y / a.gridSize))
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
	return GridCoord{X: x, Y: y}
}

func (a *Grid) addToCell(coord GridCoord, id entity.EntityID) {
	if a.cells[coord] == nil {
		a.cells[coord] = map[entity.EntityID]bool{}
	}
	a.cells[coord][id] = true
}

func (a *Grid) removeFromCell(coord GridCoord, id entity.EntityID) {
	cell := a.cells[coord]
	if cell == nil {
		return
	}
	delete(cell, id)
	if len(cell) == 0 {
		delete(a.cells, coord)
	}
}
