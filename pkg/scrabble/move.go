package scrabble

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	BoardCenter int = 7
)

// Make sure the moves structs implements Move interface
var (
	_ Move = (*TileMove)(nil)
	_ Move = (*PassMove)(nil)
	_ Move = (*ExchangeMove)(nil)
	_ Move = (*FinalMove)(nil)
)

type Move interface {
	IsValid(*Game) bool
	Apply(*Game) error
	Score(*GameState) int
	String() string // Just to print in console
}

type TileMove struct {
	Start         Position
	End           Position
	Covers        Covers
	Horizontal    bool
	Word          string
	CachedScore   *int
	ValidateWords bool // True when move is not from a bot
}

type PassMove struct{}

type ExchangeMove struct {
	Letters string
}

// FinalMove represents the final adjustments that are made to
// player scores at the end of a Game
type FinalMove struct {
	OpponentRack   string
	MultiplyFactor int
}

// Covers is a map of board coordinates to the letter covering the square
type Covers map[Position]Cover

// If Letter is a blank "*", then actual is the real letter
type Cover struct {
	Letter rune
	Actual rune
}

const BingoBonus = 50

const IllegalMoveWord string = "[???]"

func NewTileMove(b *Board, covers Covers) *TileMove {
	move := &TileMove{}
	move.Init(b, covers, true)
	return move
}

func NewUncheckedTileMove(b *Board, covers Covers) *TileMove {
	move := &TileMove{}
	move.Init(b, covers, false)
	return move
}

// Init initializes a TileMove instance for a particular Board
// using a map of Coordinate to Cover
func (move *TileMove) Init(b *Board, covers Covers, validateWords bool) {
	move.Covers = covers
	top, left := BoardSize, BoardSize
	bottom, right := -1, -1
	for pos := range covers {
		if pos.Row < top {
			top = pos.Row
		}
		if pos.Col < left {
			left = pos.Col
		}
		if pos.Row > bottom {
			bottom = pos.Row
		}
		if pos.Col > right {
			right = pos.Col
		}
	}
	move.Start = Position{Row: top, Col: left}
	move.End = Position{Row: bottom, Col: right}

	// A word is valid if at least made of two letters
	if len(covers) >= 2 {
		// This is horizontal if the first two covers are in the same row
		move.Horizontal = top == bottom
	} else {
		// Single cover: get smart and figure out whether the
		// horizontal cross is longer than the vertical cross
		hcross := len(b.TileFragment(move.Start, DirectionLeft)) +
			len(b.TileFragment(move.Start, DirectionRight))
		vcross := len(b.TileFragment(move.Start, DirectionAbove)) +
			len(b.TileFragment(move.Start, DirectionBellow))
		move.Horizontal = hcross >= vcross
	}
	// By default, words formed by tile moves need to
	// be validated. However this is turned off when
	// generating robot moves as they are valid by default.
	move.ValidateWords = validateWords
	// Collect the entire word that is being laid down
	var direction, reverse int
	if move.Horizontal {
		direction = DirectionRight
		reverse = DirectionLeft
	} else {
		direction = DirectionBellow
		reverse = DirectionAbove
	}
	sq := b.GetSquare(move.Start)
	if sq == nil {
		move.Word = IllegalMoveWord
		return
	}
	// Start with any left prefix that is being extended
	word := b.WordFragment(move.Start, reverse)
	// Next, traverse the covering line from top left to bottom right
	for {
		if cover, ok := covers[sq.Position]; ok {
			// This square is being covered by the tile move
			word += string(cover.Actual)
		} else {
			// This square must be covered by a previously laid tile
			if sq.Tile == nil {
				move.Word = IllegalMoveWord
				return
			}
			word += string(sq.Tile.Letter)
		}
		if sq.Position.Row == bottom && sq.Position.Col == right {
			// This was the last tile laid down in the move:
			// the loop is done
			break
		}
		// Move to the next adjacent square, in the direction of the move
		sq = b.Adjacents[sq.Position.Row][sq.Position.Col][direction]
		if sq == nil {
			move.Word = IllegalMoveWord
			return
		}
	}
	// Add any suffix that may already have been on the board
	word += b.WordFragment(move.End, direction)
	move.Word = word
}

// IsValid returns true if the TileMove is valid in the current Game
func (move *TileMove) IsValid(game *Game) bool {
	// Check the validity of the move
	if len(move.Covers) < 1 || len(move.Covers) > RackSize {
		return false
	}
	b := game.Board
	// Count the number of tiles adjacent to the covers
	numAdjacentTiles := 0
	for pos := range move.Covers {
		if !pos.InBounds() {
			return false
		}
		if b.GetSquare(pos).Tile != nil {
			// There is already a tile in this square
			return false
		}
		numAdjacentTiles += b.NumAdjacentTiles(pos)
	}
	if move.End.Row > move.Start.Row &&
		move.End.Col > move.Start.Col {
		// Not strictly horizontal or strictly vertical
		return false
	}
	// Check for gaps
	if move.Horizontal {
		// This is a horizontal move
		row := move.Start.Row
		for i := move.Start.Col; i <= move.End.Col; i++ {
			pos := Position{Row: row, Col: i}
			_, covered := move.Covers[pos]
			if !covered && b.GetSquare(pos).Tile == nil {
				// There is a missing square in the covers
				return false
			}
		}
	} else {
		// This is a vertical move
		col := move.Start.Col
		for i := move.Start.Row; i <= move.End.Row; i++ {
			pos := Position{Row: i, Col: col}
			_, covered := move.Covers[pos]
			if !covered && b.GetSquare(pos).Tile == nil {
				// There is a missing square in the covers
				return false
			}
		}
	}
	// The first tile move must go through the center
	centerPos := Position{Row: BoardCenter, Col: BoardCenter}
	if b.GetSquare(centerPos).Tile == nil {
		if _, covered := move.Covers[centerPos]; !covered {
			return false
		}
	} else {
		// At least one cover must touch a tile
		// that is already on the board
		if numAdjacentTiles == 0 {
			return false
		}
	}
	if !move.ValidateWords {
		// No need to validate the words formed by this move on the board:
		// return true, we're done
		return true
	}

	// Check if word is valid
	if move.Word == IllegalMoveWord || move.Word == "" {
		return false
	}
	if !game.DAWG.IsWord(move.Word) {
		return false
	}
	// Check if the cross words are valid
	for pos, coverLetter := range move.Covers {
		left, right := game.Board.CrossWordFragments(pos, !move.Horizontal)
		if len(left) > 0 || len(right) > 0 {
			// There is a cross word here: check it
			if !game.DAWG.IsWord(left + string(coverLetter.Actual) + right) {
				return false
			}
		}
	}
	return true
}

// Apply moves the tiles in the Covers from the player's Rack
// to the board Squares. Move should be valid here
func (move *TileMove) Apply(game *Game) error {
	rack := game.PlayerToMove().Rack
	for pos, cover := range move.Covers {
		var (
			tile *Tile
			err  error
		)

		tile, err = rack.GetTile(cover.Letter)
		if cover.Letter == '*' {
			// It is a blank tile, put the letter to uppercase on the tile
			tile.Letter = unicode.ToUpper(cover.Actual)
		}

		if err != nil {
			// Should not happen
			return err
		}

		err = game.PlayTile(tile, pos, rack)
		if err != nil {
			// Should not happen
			return err
		}

	}
	rack.Fill(game.Bag)
	// Reset the counter of consecutive pass moves
	game.NumPassMoves = 0
	return nil
}

// Score returns the score of the TileMove, if
// played in the given Game
func (move *TileMove) Score(state *GameState) int {
	if move.CachedScore != nil {
		return *move.CachedScore
	}
	// Cumulative letter score
	score := 0
	// Cumulative cross scores
	crossScore := 0
	// Word multiplier
	multiplier := 1
	rowIncr, colIncr := 0, 0
	var direction int
	if move.Horizontal {
		direction = DirectionLeft
		colIncr = 1
	} else {
		direction = DirectionAbove
		rowIncr = 1
	}
	// Start with tiles above the top left
	pos := move.Start

	// Add value of tiles before
	for _, tile := range state.Board.TileFragment(pos, direction) {
		score += tile.Value
	}
	// Then, progress from the top left to the bottom right
	for {
		s := state.Board.GetSquare(pos)
		if cover, covered := move.Covers[pos]; covered {
			sc := state.TileSet.Values[cover.Letter] * s.LetterMultiplier
			score += sc
			multiplier *= s.WordMultiplier
			// Add cross score, if any
			hasCrossing, csc := state.Board.CrossScore(pos, !move.Horizontal)
			if hasCrossing {
				crossScore += (csc + sc) * s.WordMultiplier
			}
		} else {
			// This square was already covered: add its letter score only
			score += s.Tile.Value
		}

		if pos.Row >= move.End.Row && pos.Col >= move.End.Col {
			break
		}
		pos.Row += rowIncr
		pos.Col += colIncr
	}

	// Finally, add tiles below the bottom right
	pos = move.End
	if move.Horizontal {
		direction = DirectionRight
	} else {
		direction = DirectionBellow
	}
	for _, tile := range state.Board.TileFragment(pos, direction) {
		score += tile.Value
	}
	// Multiply the accumulated letter score with the word multiplier
	score *= multiplier
	// Add cross scores
	score += crossScore
	if len(move.Covers) == RackSize {
		// The player played his entire rack: add the bingo bonus
		score += BingoBonus
	}
	// Only calculate the score once, then cache it
	move.CachedScore = &score
	return score
}

func (move *TileMove) String() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Start%v End%v Word: %s Score: %d", move.Start, move.End, move.Word, *move.CachedScore))

	return sb.String()
}

// NewPassMove returns a reference to a fresh PassMove
func NewPassMove() *PassMove {
	return &PassMove{}
}

// IsValid always returns true for a PassMove
func (move *PassMove) IsValid(game *Game) bool {
	return true
}

func (move *PassMove) Apply(game *Game) error {
	// Increment the number of consecutive pass moves
	game.NumPassMoves++
	return nil
}

// Score is always 0 for a PassMove
func (move *PassMove) Score(state *GameState) int {
	return 0
}

// String return a string description of the PassMove
func (move *PassMove) String() string {
	return "Pass"
}

// NewExchangeMove returns a reference to a fresh ExchangeMove
func NewExchangeMove(letters string) *ExchangeMove {
	return &ExchangeMove{Letters: letters}
}

// IsValid returns true if an exchange is allowed and all
// exchanged tiles are actually in the player's rack
func (move *ExchangeMove) IsValid(game *Game) bool {
	if move == nil || game == nil {
		return false
	}
	if !game.Bag.ExchangeAllowed() {
		// Too few tiles left in the bag
		return false
	}
	runes := []rune(move.Letters)
	if len(runes) < 1 || len(runes) > RackSize {
		return false
	}
	rack := game.PlayerToMove().Rack.AsString()
	for _, letter := range runes {
		if !strings.ContainsRune(rack, letter) {
			// This exchanged letter is not in the player's rack
			return false
		}
		rack = strings.Replace(rack, string(letter), "", 1)
	}
	// All exchanged letters found: the move is OK
	return true
}

// Apply replenishes the exchanged tiles in the Rack
// from the Bag
func (move *ExchangeMove) Apply(game *Game) error {
	rack := game.PlayerToMove().Rack
	tiles := make([]*Tile, 0, RackSize)
	// First, remove the exchanged tiles from the player's Rack
	for _, letter := range move.Letters {
		tile, err := rack.GetTile(letter)
		if err != nil {
			// Should not happen because moves.Letters are built using
			// rack's tiles
			return err
		}
		if err = rack.Remove(tile.Letter); err != nil {
			// Should not happen because moves.Letters are built using
			// rack's tiles
			return err
		}
		tiles = append(tiles, tile)
	}
	// Replenish the Rack from the Bag...
	rack.Fill(game.Bag)
	// ...before returning the exchanged tiles to the Bag
	for _, tile := range tiles {
		game.Bag.ReturnTile(tile)
	}
	// Increment the number of consecutive pass moves
	game.NumPassMoves++
	return nil
}

// Score is always 0 for an ExchangeMove
func (move *ExchangeMove) Score(state *GameState) int {
	return 0
}

// String return a string description of the ExchangeMove
func (move *ExchangeMove) String() string {
	return "Exchanged letters: " + move.Letters
}

func NewFinalMove(rackOpp string, multiplyFactor int) *FinalMove {
	return &FinalMove{OpponentRack: rackOpp, MultiplyFactor: multiplyFactor}
}

func (move *FinalMove) IsValid(game *Game) bool {
	return true
}

func (move *FinalMove) Apply(game *Game) error {
	return nil
}

// Score returns the opponent's rack leave, multiplied
// by a multiplication factor that can be 1 or 2
func (move *FinalMove) Score(state *GameState) int {
	adj := 0
	for _, letter := range move.OpponentRack {
		adj += state.TileSet.Values[letter]
	}
	return adj * move.MultiplyFactor
}

// String return a string description of the FinalMove
func (move *FinalMove) String() string {
	return "Rack " + move.OpponentRack
}
