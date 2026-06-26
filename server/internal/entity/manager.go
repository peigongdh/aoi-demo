package entity

type Manager struct {
	entities map[EntityID]Entity
}

func NewManager() *Manager {
	return &Manager{entities: map[EntityID]Entity{}}
}

func (m *Manager) Add(entity Entity) {
	m.entities[entity.ID()] = entity
}

func (m *Manager) Remove(id EntityID) {
	delete(m.entities, id)
}

func (m *Manager) Get(id EntityID) Entity {
	return m.entities[id]
}

func (m *Manager) All() []Entity {
	out := make([]Entity, 0, len(m.entities))
	for _, entity := range m.entities {
		out = append(out, entity)
	}
	return out
}

func (m *Manager) Players() []*Player {
	out := []*Player{}
	for _, entity := range m.entities {
		if player, ok := entity.(*Player); ok {
			out = append(out, player)
		}
	}
	return out
}

func (m *Manager) NPCs() []*NPC {
	out := []*NPC{}
	for _, entity := range m.entities {
		if npc, ok := entity.(*NPC); ok {
			out = append(out, npc)
		}
	}
	return out
}
