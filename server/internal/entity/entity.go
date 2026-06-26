package entity

import "math"

type EntityID string
type EntityType string

const (
	EntityPlayer EntityType = "player"
	EntityNPC    EntityType = "npc"
)

type Vec2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (v Vec2) Add(other Vec2) Vec2 {
	return Vec2{X: v.X + other.X, Y: v.Y + other.Y}
}

func (v Vec2) Scale(s float64) Vec2 {
	return Vec2{X: v.X * s, Y: v.Y * s}
}

func (v Vec2) Len() float64 {
	return math.Hypot(v.X, v.Y)
}

func (v Vec2) Normalize() Vec2 {
	length := v.Len()
	if length == 0 {
		return Vec2{}
	}
	return Vec2{X: v.X / length, Y: v.Y / length}
}

func Distance(a, b Vec2) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}

type Entity interface {
	ID() EntityID
	Type() EntityType
	Position() Vec2
	SetPosition(Vec2)
	Velocity() Vec2
	SetVelocity(Vec2)
	Radius() float64
}

type BaseEntity struct {
	Id      EntityID   `json:"id"`
	Kind    EntityType `json:"type"`
	Pos     Vec2       `json:"pos"`
	Vel     Vec2       `json:"velocity"`
	RadiusV float64    `json:"radius"`
}

func (e *BaseEntity) ID() EntityID {
	return e.Id
}

func (e *BaseEntity) Type() EntityType {
	return e.Kind
}

func (e *BaseEntity) Position() Vec2 {
	return e.Pos
}

func (e *BaseEntity) SetPosition(pos Vec2) {
	e.Pos = pos
}

func (e *BaseEntity) Velocity() Vec2 {
	return e.Vel
}

func (e *BaseEntity) SetVelocity(vel Vec2) {
	e.Vel = vel
}

func (e *BaseEntity) Radius() float64 {
	return e.RadiusV
}

type Player struct {
	BaseEntity
	ConnID       string
	Name         string
	AOIRadius    float64
	VisibleSet   map[EntityID]bool
	LastInputSeq uint64
}

type NPCBehavior string

const (
	NPCStatic NPCBehavior = "static"
	NPCWander NPCBehavior = "wander"
)

type NPC struct {
	BaseEntity
	Behavior     NPCBehavior
	NextTurnInMS int
}
