package scrabble

var tileValue = map[rune]int{
	'e': 1,
	'a': 1,
	'i': 1,
	'n': 1,
	'o': 1,
	'r': 1,
	's': 1,
	't': 1,
	'u': 1,
	'l': 1,
	'd': 2,
	'm': 2,
	'g': 2,
	'b': 3,
	'c': 3,
	'p': 3,
	'f': 4,
	'h': 4,
	'v': 4,
	'j': 8,
	'q': 8,
	'k': 10,
	'w': 10,
	'x': 10,
	'y': 10,
	'z': 10,
	'*': 0,
}

type Tile struct {
	Letter rune
	Value  int
}

func NewTile(char rune) *Tile {
	return &Tile{
		Letter: char,
		Value:  tileValue[char],
	}
}
