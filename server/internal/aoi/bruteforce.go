package aoi

import "aoi-demo/server/internal/entity"

type BruteForce struct {
	baseManager
}

func NewBruteForce() *BruteForce {
	return &BruteForce{baseManager: newBaseManager()}
}

func (a *BruteForce) Name() string {
	return "bruteforce"
}

func (a *BruteForce) Query(observerID entity.EntityID, radius float64) ([]entity.EntityID, error) {
	observer := a.entities[observerID]
	if observer == nil {
		return nil, nil
	}

	visible := map[entity.EntityID]bool{}
	for id, target := range a.entities {
		if id == observerID {
			continue
		}
		if entity.Distance(observer.Position(), target.Position()) <= radius {
			visible[id] = true
		}
	}
	return sortedIDs(visible), nil
}

func (a *BruteForce) Update(world WorldState) ([]Event, error) {
	return updateByQuery(a, world)
}
