package game

import "fmt"

type History struct {
	exchanges []string
	maxSize   int
}

func NewHistory(maxSize int) *History {
	return &History{
		exchanges: make([]string, 0, maxSize),
		maxSize:   maxSize,
	}
}

func (h *History) AddPlayerAction(input string) {
	h.add("Player: " + input)
}

func (h *History) AddNarratorResponse(response string) {
	h.add("Narrator: " + response)
}

func (h *History) AddError(err error) {
	h.add("Error: " + err.Error())
}

func (h *History) add(entry string) {
	h.exchanges = append(h.exchanges, entry)
	
	if len(h.exchanges) > h.maxSize {
		h.exchanges = h.exchanges[len(h.exchanges)-h.maxSize:]
	}
}

func (h *History) GetEntries() []string {
	result := make([]string, len(h.exchanges))
	copy(result, h.exchanges)
	return result
}

func (h *History) BuildContext(world WorldState) string {
	currentLoc := world.Locations[world.Location]
	context := fmt.Sprintf(`WORLD STATE:
Current Location: %s (%s)
%s

Available Items Here: %v
Available Exits: %v
Player Inventory: %v

`, currentLoc.Title, world.Location, currentLoc.Description, currentLoc.Items, currentLoc.Exits, world.Inventory)

	if len(h.exchanges) > 0 {
		context += "RECENT CONVERSATION:\n"
		for _, exchange := range h.exchanges {
			context += exchange + "\n"
		}
		context += "\n"
	}

	return context
}