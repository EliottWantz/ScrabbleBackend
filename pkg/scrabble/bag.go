package scrabble

import (
	"errors"
	"math/rand"
)

const TotalTiles int = 102

var ErrBagEmpty = errors.New("bag is empty")

type Bag struct {
	Tiles []Tile

	TileSet *TileSet
}

type TileSet struct {
	Count  map[rune]int
	Values map[rune]int
}

func initTileSet() *TileSet {
	tileCount := map[rune]int{
		'a': 9, 'b': 2, 'c': 2, 'd': 3, 'e': 15,
		'f': 2, 'g': 2, 'h': 2, 'i': 8, 'j': 1,
		'k': 1, 'l': 5, 'm': 3, 'n': 6, 'o': 6,
		'p': 2, 'q': 1, 'r': 6, 's': 6, 't': 6,
		'u': 6, 'v': 2, 'w': 1, 'x': 1, 'y': 1,
		'z': 1, '*': 2,
	}

	tileValue := map[rune]int{
		'a': 1, 'b': 3, 'c': 3, 'd': 2, 'e': 1,
		'f': 4, 'g': 2, 'h': 4, 'i': 1, 'j': 8,
		'k': 10, 'l': 1, 'm': 2, 'n': 1, 'o': 1,
		'p': 3, 'q': 8, 'r': 1, 's': 1, 't': 1,
		'u': 1, 'v': 4, 'w': 10, 'x': 10, 'y': 10,
		'z': 10, '*': 0,
	}

	return &TileSet{Count: tileCount, Values: tileValue}
}

var DefaultTileSet = initTileSet()

func NewBag(tileset *TileSet) *Bag {
	b := &Bag{
		Tiles:   make([]Tile, 0, TotalTiles),
		TileSet: tileset,
	}

	for letter, count := range tileset.Count {
		for i := 0; i < count; i++ {
			b.Tiles = append(b.Tiles,
				Tile{
					Letter: letter,
					Value:  b.TileSet.Values[letter],
				},
			)
		}
	}

	b.shuffle()

	return b
}

func (b *Bag) shuffle() {
	rand.Shuffle(b.TileCount(), func(i, j int) {
		b.Tiles[i], b.Tiles[j] = b.Tiles[j], b.Tiles[i]
	})
}

func (b *Bag) TileCount() int {
	return len(b.Tiles)
}

func (b *Bag) DrawTile() (*Tile, error) {
	tileCount := b.TileCount()

	if tileCount == 0 {
		return nil, ErrBagEmpty
	}

	// # nosec
	i := rand.Intn(tileCount)
	tile := b.Tiles[i]

	b.RemoveTile(i)

	return &tile, nil
}

func (b *Bag) RemoveTile(i int) {
	// No need to keep order in bag
	end := b.TileCount() - 1
	b.Tiles[i] = b.Tiles[end]
	b.Tiles = b.Tiles[:end]

	// Keep order
	// b.Contents = append(b.Contents[:i], b.Contents[i+1:]...)
}

func (b *Bag) ReturnTile(t *Tile) {
	b.Tiles = append(b.Tiles, *t)
}

func (bag *Bag) ExchangeAllowed() bool {
	return bag.TileCount() >= RackSize
}
