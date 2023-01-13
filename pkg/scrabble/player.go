package scrabble

import (
	"github.com/google/uuid"
)

type Player struct {
	ID       uuid.UUID
	Username string
	Rack     *Rack
	Score    int
}

func NewPlayer(username string, b *Bag) *Player {
	return &Player{
		ID:       uuid.New(),
		Username: username,
		Rack:     NewRack(b),
	}
}
