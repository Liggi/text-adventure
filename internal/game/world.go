package game

import "strings"

type WorldState struct {
	Location  string
	Inventory []string
	MetNPCs   []string
	Locations map[string]LocationInfo
	NPCs      map[string]NPCInfo
}

type LocationInfo struct {
	Name        string
	Exits       map[string]string
	Facts       []string
}

type NPCInfo struct {
	Location      string
	DebugColor    string
	Description   string
	Inventory     []string
	RecentThoughts []string
	RecentActions []string
	Personality   string
	Backstory     string
	Memories      []string
	Facts         []string
}

type ItemInfo struct {
	Name     string
	Facts    []string
	Location string
}

func NewDefaultWorldState() WorldState {
	return WorldState{
		Location:  "foyer",
		Inventory: []string{},
		Locations: map[string]LocationInfo{
			"foyer": {
				Name:  "foyer",
				Facts: []string{},
				Exits: map[string]string{"north": "study", "east": "library", "west": "kitchen"},
			},
			"study": {
				Name:  "study",
				Facts: []string{},
				Exits: map[string]string{"south": "foyer"},
			},
			"library": {
				Name:  "library",
				Facts: []string{},
				Exits: map[string]string{"west": "foyer"},
			},
			"kitchen": {
				Name:  "kitchen",
				Facts: []string{},
				Exits: map[string]string{"east": "foyer"},
			},
		},
		NPCs: map[string]NPCInfo{
			"elena": {
				Location:    "library",
				Personality: "cautious, observant, struggling with disorientation",
				Backstory:   "recently awakened in this strange place with no memory of how she got here or who she was before",
				Memories: []string{
					"woke up somewhere unfamiliar",
					"has no memory of her past",
					"feeling disoriented and cautious",
				},
				Facts:           []string{},
				RecentThoughts:  []string{},
				RecentActions:   []string{},
				Inventory:       []string{},
				DebugColor:      "yellow",
				Description:     "someone",
			},
		},
	}
}

func (ws *WorldState) AccumulateLocationFacts(locationID string, newFacts []string) {
	if len(newFacts) == 0 {
		return
	}
	
	loc, exists := ws.Locations[locationID]
	if !exists {
		return
	}
	
	for _, newFact := range newFacts {
		newFact = strings.TrimSpace(newFact)
		if newFact == "" {
			continue
		}
		
		duplicate := false
		for _, existingFact := range loc.Facts {
			if existingFact == newFact {
				duplicate = true
				break
			}
		}
		
		if !duplicate {
			loc.Facts = append(loc.Facts, newFact)
		}
	}
	
	ws.Locations[locationID] = loc
}