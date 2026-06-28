package protocol

import (
	"encoding/json"

	"aoi-demo/server/internal/entity"
)

const (
	TypeJoinWorld    = "join_world"
	TypeInput        = "input"
	TypePing         = "ping"
	TypePong         = "pong"
	TypeWelcome      = "welcome"
	TypeEntityEnter  = "entity_enter"
	TypeEntityLeave  = "entity_leave"
	TypeEntityUpdate = "entity_update"
)

type IncomingMessage struct {
	Type    string          `json:"type"`
	Seq     uint64          `json:"seq,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type Message struct {
	Type    string `json:"type"`
	Seq     uint64 `json:"seq,omitempty"`
	Payload any    `json:"payload,omitempty"`
}

type JoinWorldPayload struct {
	Name string `json:"name"`
}

type InputPayload struct {
	Up    bool `json:"up"`
	Down  bool `json:"down"`
	Left  bool `json:"left"`
	Right bool `json:"right"`
}

type PingPayload struct {
	ClientTime float64 `json:"client_time"`
}

type PongPayload struct {
	ClientTime float64 `json:"client_time"`
	ServerTime int64   `json:"server_time"`
}

type WelcomePayload struct {
	PlayerID    entity.EntityID `json:"player_id"`
	MapWidth    float64         `json:"map_width"`
	MapHeight   float64         `json:"map_height"`
	AOIType     string          `json:"aoi_type"`
	AOIRadius   float64         `json:"aoi_radius"`
	GridSize    float64         `json:"grid_size"`
	SyncType    string          `json:"sync_type"`
	PlayerSpeed float64         `json:"player_speed"`
	ServerTick  uint64          `json:"server_tick"`
	Entity      EntityState     `json:"entity"`
}

type EntityState struct {
	ID           entity.EntityID   `json:"id"`
	Type         entity.EntityType `json:"type"`
	Name         string            `json:"name,omitempty"`
	X            float64           `json:"x"`
	Y            float64           `json:"y"`
	Radius       float64           `json:"radius"`
	LastInputSeq uint64            `json:"last_input_seq,omitempty"`
}

type EntityEnterPayload struct {
	Entity EntityState `json:"entity"`
}

type EntityLeavePayload struct {
	EntityID entity.EntityID `json:"entity_id"`
}

type EntityUpdatePayload struct {
	Entities      []EntityState `json:"entities"`
	DebugEntities []EntityState `json:"debug_entities,omitempty"`
	ServerTick    uint64        `json:"server_tick"`
	ServerTime    int64         `json:"server_time"`
	SyncType      string        `json:"sync_type"`
	FullSnapshot  bool          `json:"full_snapshot"`
	Stats         ServerStats   `json:"stats"`
}

type ServerStats struct {
	PlayerCount    int     `json:"player_count"`
	NPCCount       int     `json:"npc_count"`
	EntityCount    int     `json:"entity_count"`
	SyncType       string  `json:"sync_type"`
	AOIQueryMS     float64 `json:"aoi_query_ms"`
	TickDurationMS float64 `json:"tick_duration_ms"`
	MessagesOut    int     `json:"messages_out"`
	AOIEvents      int     `json:"aoi_events"`
}
