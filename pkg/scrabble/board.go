package scrabble

import (
	"errors"
	"fmt"
	"strings"
)

const BoardSize int = 15

var (
	ErrInvalidPosition                  = errors.New("position is out of bounds")
	ErrInvalidPlacementOnNonEmptySquare = errors.New("cannot place a tile on a square that already has a tile on it")
)

var (
	wordMultipliers = [BoardSize]string{
		"311111131111113",
		"121111111111121",
		"112111111111211",
		"111211111112111",
		"111121111121111",
		"111111111111111",
		"111111111111111",
		"311111121111113",
		"111111111111111",
		"111111111111111",
		"111121111121111",
		"111211111112111",
		"112111111111211",
		"121111111111121",
		"311111131111113",
	}

	letterMultipliers = [BoardSize]string{
		"111211111112111",
		"111113111311111",
		"111111212111111",
		"211111121111112",
		"111111111111111",
		"131113111311131",
		"112111212111211",
		"111211111112111",
		"112111212111211",
		"131113111311131",
		"111111111111111",
		"211111121111112",
		"111111212111111",
		"111113111311111",
		"111211111112111",
	}

	// row/col Ids are the row identifiers of a board, for printing purposes
	rowIds = [BoardSize]string{
		"0", "1", "2", "3", "4",
		"5", "6", "7", "8", "9",
		"0", "1", "2", "3", "4",
	}
)

type Board struct {
	Squares   [BoardSize][BoardSize]Square
	Adjacents [BoardSize][BoardSize][4]*Square
}

type Square struct {
	Tile             *Tile
	LetterMultiplier int
	WordMultiplier   int
	Position         Position
}

type Position struct {
	Row, Col int
}

type Direction = int

const (
	DirectionAbove Direction = iota
	DirectionLeft
	DirectionRight
	DirectionBellow
)

func NewBoard() *Board {
	b := &Board{}

	const zeroUnicode = '0'
	for i := 0; i < BoardSize; i++ {
		for j := 0; j < BoardSize; j++ {
			b.Squares[i][j] = Square{
				LetterMultiplier: int(letterMultipliers[i][j] - zeroUnicode),
				WordMultiplier:   int(wordMultipliers[i][j] - zeroUnicode),
				Position: Position{
					Row: i,
					Col: j,
				},
			}
		}
	}

	// Initialize the adjacent square lists
	for row := 0; row < BoardSize; row++ {
		for col := 0; col < BoardSize; col++ {
			adj := &b.Adjacents[row][col]
			if row > 0 {
				adj[DirectionAbove] = b.GetSquare(
					Position{
						Row: row - 1,
						Col: col,
					},
				)
			}
			if row < BoardSize-1 {
				adj[DirectionBellow] = b.GetSquare(
					Position{
						Row: row + 1,
						Col: col,
					},
				)
			}
			if col > 0 {
				adj[DirectionLeft] = b.GetSquare(
					Position{
						Row: row,
						Col: col - 1,
					},
				)
			}
			if col < BoardSize-1 {
				adj[DirectionRight] = b.GetSquare(
					Position{
						Row: row,
						Col: col + 1,
					},
				)
			}
		}
	}

	return b
}

func (b *Board) GetSquare(p Position) *Square {
	return &b.Squares[p.Row][p.Col]
}

func (b *Board) PlaceTile(p Position, t *Tile) error {
	sq := b.GetSquare(p)

	if sq.Tile != nil {
		return ErrInvalidPlacementOnNonEmptySquare
	}

	sq.Tile = t

	return nil
}

// TileFragment returns a list of the tiles that extend from the square
// at given pos in the direction specified.
func (board *Board) TileFragment(pos Position, direction Direction) []*Tile {
	if !pos.InBounds() {
		return nil
	}
	if direction < DirectionAbove || direction > DirectionBellow {
		return nil
	}

	frag := make([]*Tile, 0, BoardSize-1)
	for {
		sq := board.Adjacents[pos.Row][pos.Col][direction]
		// If there is no adjacent square in direction, than can't be
		// more letters in that direction
		if sq == nil || sq.Tile == nil {
			break
		}
		frag = append(frag, sq.Tile)
		pos = sq.Position
	}

	return frag
}

// WordFragment returns the word formed by the tile sequence emanating
// from the given square in the indicated direction, not including the
// square itself.
func (board *Board) WordFragment(pos Position, direction Direction) string {
	result := ""
	frag := board.TileFragment(pos, direction)

	if direction == DirectionLeft || direction == DirectionAbove {
		// We need to reverse the order of the fragment
		for _, tile := range frag {
			result = string(tile.Letter) + result
		}
	} else {
		// The fragment is in correct reading order
		for _, tile := range frag {
			result += string(tile.Letter)
		}
	}
	return result
}

// CrossWordFragments returns the word fragments above and below (vertical),
// or to the left and right (horizontal), of the given position on the board.
func (board *Board) CrossWordFragments(pos Position, horizontal bool) (prev, after string) {
	var direction int

	if horizontal {
		direction = DirectionLeft
	} else {
		direction = DirectionAbove
	}

	prev = board.WordFragment(pos, direction)

	if horizontal {
		direction = DirectionRight
	} else {
		direction = DirectionBellow
	}

	after = board.WordFragment(pos, direction)

	return prev, after
}

// NumAdjacentTiles returns the number of tiles on the
// Board that are adjacent to the given coordinate
func (board *Board) NumAdjacentTiles(pos Position) int {
	adj := &board.Adjacents[pos.Row][pos.Col]
	count := 0
	for _, sq := range adj {
		if sq != nil && sq.Tile != nil {
			count++
		}
	}
	return count
}

// CrossScore returns the sum of the scores of the tiles crossing
// the given tile, either horizontally or vertically. If there are no
// crossings, returns false, 0. (Note that true, 0 is a valid return
// value, if a crossing has only blank tiles.)
func (board *Board) CrossScore(pos Position, horizontal bool) (hasCrossing bool, score int) {
	var direction int
	if horizontal {
		direction = DirectionLeft
	} else {
		direction = DirectionAbove
	}

	for _, tile := range board.TileFragment(pos, direction) {
		score += tile.Value
		hasCrossing = true
	}

	if horizontal {
		direction = DirectionRight
	} else {
		direction = DirectionBellow
	}

	for _, tile := range board.TileFragment(pos, direction) {
		score += tile.Value
		hasCrossing = true
	}
	return hasCrossing, score
}

// String represents a Board as a string
func (b *Board) String() string {
	var sb strings.Builder
	sb.WriteString("  ")
	for i := 0; i < BoardSize; i++ {
		sb.WriteString(rowIds[i] + " ")
	}
	sb.WriteString("\n")
	for i := 0; i < BoardSize; i++ {
		sb.WriteString(fmt.Sprintf("%s ", rowIds[i]))
		for j := 0; j < BoardSize; j++ {
			sq := b.GetSquare(Position{i, j})
			sb.WriteString(fmt.Sprintf("%v ", sq))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (s *Square) String() string {
	if s.Tile == nil {
		return "-"
	}
	return string(s.Tile.Letter)
}

func (s *Square) IsEmpty() bool {
	return s.Tile == nil
}

// A square is an anchor if there is no tile on it and at least one of it's
// neighbors is not empty
func (s *Square) IsAnchor(b *Board) bool {
	if !s.IsEmpty() {
		return false
	}

	for _, adj := range b.Adjacents[s.Position.Row][s.Position.Col] {
		if adj != nil && adj.Tile != nil {
			return true
		}
	}

	return false
}

func (p Position) InBounds() bool {
	if p.Row < 0 {
		return false
	}
	if p.Row >= BoardSize {
		return false
	}
	if p.Col < 0 {
		return false
	}
	if p.Col >= BoardSize {
		return false
	}

	return true
}
