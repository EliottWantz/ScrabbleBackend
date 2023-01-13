package scrabble

import "errors"

const (
	RackSize = 7
)

var ErrTileNotInRack = errors.New("tile not in rack")

type Rack struct {
	Tiles []*Tile
}

func NewRack(b *Bag) *Rack {
	rack := &Rack{
		Tiles: make([]*Tile, 0, RackSize),
	}
	rack.Fill(b)

	return rack
}

func (r *Rack) Fill(b *Bag) {
	for len(r.Tiles) != RackSize {
		tile, err := b.DrawTile()
		if err != nil {
			return
		}
		r.Tiles = append(r.Tiles, tile)
	}
}

func (r *Rack) Index(letter rune) int {
	for i, t := range r.Tiles {
		if letter == t.Letter {
			return i
		}
	}
	return -1
}

func (r *Rack) Contains(letter rune) bool {
	return r.Index(letter) >= 0
}

func (r *Rack) Remove(letter rune) error {
	i := r.Index(letter)

	if i == -1 {
		return ErrTileNotInRack
	}
	// Keep order when deleting so that when displayed on the frontend
	// the user sees the same order as before
	r.Tiles = append(r.Tiles[:i], r.Tiles[i+1:]...)

	return nil
}

func (r *Rack) GetTile(letter rune) (*Tile, error) {
	i := r.Index(letter)
	if i == -1 {
		return nil, ErrTileNotInRack
	}

	return r.Tiles[i], nil
}

func (r *Rack) AsRunes() []rune {
	var letters []rune
	for _, t := range r.Tiles {
		letters = append(letters, t.Letter)
	}

	return letters
}

func (r *Rack) AsString() string {
	return string(r.AsRunes())
}

func (r *Rack) IsEmpty() bool {
	return len(r.Tiles) == 0
}
