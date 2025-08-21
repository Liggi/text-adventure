package game

type WorldState struct {
	Location  string
	Inventory []string
	Locations map[string]LocationInfo
	NPCs      map[string]NPCInfo
}

type LocationInfo struct {
	Title       string
	Description string
	Items       []string
	Exits       map[string]string
}

type NPCInfo struct {
	Location      string
	DebugColor    string
	Inventory     []string
	RecentThoughts []string
	RecentActions []string
	Personality   string
	Backstory     string
	CoreMemories  []string
}

func NewDefaultWorldState() WorldState {
	return WorldState{
		Location:  "foyer",
		Inventory: []string{},
		Locations: map[string]LocationInfo{
			"foyer": {
				Title:       "Old Foyer",
				Description: "A dusty foyer with motes drifting in shafts of light",
				Items:       []string{"silver key"},
				Exits:       map[string]string{"north": "study"},
			},
			"study": {
				Title:       "Quiet Study",
				Description: "A quiet study with a heavy oak desk",
				Items:       []string{},
				Exits:       map[string]string{"south": "foyer"},
			},
		},
	}
}