package scrabble

import (
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
)

const (
	BoardCenter int = 7
)

type Move interface {
	IsValid(*Game) bool
	Apply(*Game) bool
	Score(*GameState) int
	String() string
}

// FinalMove represents the final adjustments that are made to
// player scores at the end of a Game
type FinalMove struct {
	OpponentRack   string
	MultiplyFactor int
}

// Make sure the PassMove implements Move interface
var _ Move = (*FinalMove)(nil)

// PassMove is a move that is always valid, has no effect when applied,
// and has a score of 0
type PassMove struct{}

// Make sure the PassMove implements Move interface
var _ Move = (*PassMove)(nil)

// ExchangeMove is a move that exchanges 1-7 tiles from the player's
// Rack with the Bag. It is only valid when at least 7 tiles are
// left in the Bag.
type ExchangeMove struct {
	Letters string
}

// Make sure the ExchangeMove implements Move interface
var _ Move = (*ExchangeMove)(nil)

type TileMove struct {
	TopLeft       Position
	BottomRight   Position
	Covers        Covers
	Horizontal    bool
	Word          string
	CachedScore   *int
	ValidateWords bool // True when move is not from a bot
}

// Make sure the TileMove implements Move interface
var _ Move = (*TileMove)(nil)

// Covers is a map of board coordinates to the letter covering the square
type Covers map[Position]rune

// BingoBonus is the number of extra points awarded for laying down
// all the 7 tiles in the rack in one move
const BingoBonus = 50

// IllegalMoveWord is the move.Word of an illegal move
const IllegalMoveWord string = "[???]"

func NewTileMove(board *Board, covers Covers) *TileMove {
	move := &TileMove{}
	move.Init(board, covers, true)
	return move
}

func NewUncheckedTileMove(board *Board, covers Covers) *TileMove {
	move := &TileMove{}
	move.Init(board, covers, false)
	return move
}

// IsValid returns true if the TileMove is valid in the current Game
func (move *TileMove) IsValid(game *Game) bool {
	// Check the validity of the move
	if len(move.Covers) < 1 || len(move.Covers) > RackSize {
		return false
	}
	board := game.Board
	// Count the number of tiles adjacent to the covers
	numAdjacentTiles := 0
	for pos := range move.Covers {
		if pos.Row < 0 || pos.Row >= BoardSize ||
			pos.Col < 0 || pos.Col >= BoardSize {
			return false
		}
		if board.GetSquare(pos).Tile != nil {
			// There is already a tile in this square
			return false
		}
		numAdjacentTiles += board.NumAdjacentTiles(pos)
	}
	if move.BottomRight.Row > move.TopLeft.Row &&
		move.BottomRight.Col > move.TopLeft.Col {
		// Not strictly horizontal or strictly vertical
		return false
	}
	// Check for gaps
	if move.Horizontal {
		// This is a horizontal move
		row := move.TopLeft.Row
		for i := move.TopLeft.Col; i <= move.BottomRight.Col; i++ {
			pos := Position{Row: row, Col: i}
			_, covered := move.Covers[pos]
			if !covered && board.GetSquare(pos).Tile == nil {
				// There is a missing square in the covers
				return false
			}
		}
	} else {
		// This is a vertical move
		col := move.TopLeft.Col
		for i := move.TopLeft.Row; i <= move.BottomRight.Row; i++ {
			pos := Position{Row: i, Col: col}
			_, covered := move.Covers[pos]
			if !covered && board.GetSquare(pos).Tile == nil {
				// There is a missing square in the covers
				return false
			}
		}
	}
	// The first tile move must go through the center
	centerPos := Position{Row: BoardCenter, Col: BoardCenter}
	if board.GetSquare(centerPos).Tile == nil {
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
	if move.Word == IllegalMoveWord || move.Word == "" {
		return false
	}
	if !game.DAWG.Find(move.Word) {
		return false
	}
	// Check the cross words
	for pos, coverLetter := range move.Covers {
		left, right := game.Board.CrossWordFragments(pos, !move.Horizontal)
		if len(left) > 0 || len(right) > 0 {
			// There is a cross word here: check it
			if !game.DAWG.Find(left + string(coverLetter) + right) {
				// Not found in the dictionary
				return false
			}
		}
	}
	return true
}

// Apply moves the tiles in the Covers from the player's Rack
// to the board Squares
func (move *TileMove) Apply(game *Game) bool {
	// The move is assumed to have already been validated via Move.IsValid()
	rack := game.PlayerToMove().Rack
	for pos, coverLetter := range move.Covers {
		// Find the tile in the player's rack
		tile, err := rack.GetTile(coverLetter)
		if err != nil {
			// Not found: abort
			return false
		}
		if !game.PlayTile(tile, pos) {
			// The tile was not found in the player's rack.
			// This is not good as the move may have been only partially applied.
			return false
		}
	}
	// Reset the counter of consecutive zero-point moves
	game.NumPassMoves = 0
	return true
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
	pos := move.TopLeft

	for _, tile := range state.Board.TileFragment(pos, direction) {
		score += tile.Value
	}
	// Then, progress from the top left to the bottom right
	for {
		sq := state.Board.GetSquare(pos)
		if sq == nil {
			break
		}
		if cover, covered := move.Covers[pos]; covered {
			// This square is covered by the move: apply its letter
			// and word multipliers
			thisScore := state.TileSet.Values[cover] * sq.LetterMultiplier
			score += thisScore
			multiplier *= sq.WordMultiplier
			// Add cross score, if any
			hasCrossing, csc := state.Board.CrossScore(pos, !move.Horizontal)
			if hasCrossing {
				crossScore += (csc + thisScore) * sq.WordMultiplier
			}
		} else {
			// This square was already covered: add its letter score only
			score += sq.Tile.Value
		}

		if pos.Row >= move.BottomRight.Row && pos.Col >= move.BottomRight.Col {
			break
		}
		pos.Row += rowIncr
		pos.Col += colIncr
	}

	// Finally, add tiles below the bottom right
	pos = move.BottomRight
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

// Init initializes a TileMove instance for a particular Board
// using a map of Coordinate to Cover
func (move *TileMove) Init(board *Board, covers Covers, validateWords bool) {
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
	move.TopLeft = Position{Row: top, Col: left}
	move.BottomRight = Position{Row: bottom, Col: right}

	// A word is valid if at least made of two letters
	if len(covers) >= 2 {
		// This is horizontal if the first two covers are in the same row
		move.Horizontal = top == bottom
	} else {
		// Single cover: get smart and figure out whether the
		// horizontal cross is longer than the vertical cross
		hcross := len(board.TileFragment(move.TopLeft, DirectionLeft)) +
			len(board.TileFragment(move.TopLeft, DirectionRight))
		vcross := len(board.TileFragment(move.TopLeft, DirectionAbove)) +
			len(board.TileFragment(move.TopLeft, DirectionBellow))
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
	sq := board.GetSquare(move.TopLeft)
	if sq == nil {
		move.Word = IllegalMoveWord
		return
	}
	// Start with any left prefix that is being extended
	word := board.WordFragment(move.TopLeft, reverse)
	// Next, traverse the covering line from top left to bottom right
	for {
		if letter, ok := covers[sq.Position]; ok {
			// This square is being covered by the tile move
			word += string(letter)
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
		sq = board.Adjacents[sq.Position.Row][sq.Position.Col][direction]
		if sq == nil {
			move.Word = IllegalMoveWord
			return
		}
	}
	// Add any suffix that may already have been on the board
	word += board.WordFragment(move.BottomRight, direction)
	move.Word = word
}

func (move *TileMove) String() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("TopLeft%v BottomRight%v Word: %s Score: %d", move.TopLeft, move.BottomRight, move.Word, *move.CachedScore))

	return sb.String()
}

// NewPassMove returns a reference to a fresh PassMove
func NewPassMove() *PassMove {
	return &PassMove{}
}

// String return a string description of the PassMove
func (move *PassMove) String() string {
	return "Pass"
}

// IsValid always returns true for a PassMove
func (move *PassMove) IsValid(game *Game) bool {
	return true
}

// Apply always succeeds and returns true for a PassMove
func (move *PassMove) Apply(game *Game) bool {
	// Increment the number of consecutive zero-point moves
	game.NumPassMoves++
	return true
}

// Score is always 0 for a PassMove
func (move *PassMove) Score(state *GameState) int {
	return 0
}

// NewExchangeMove returns a reference to a fresh ExchangeMove
func NewExchangeMove(letters string) *ExchangeMove {
	return &ExchangeMove{Letters: letters}
}

// String return a string description of the ExchangeMove
func (move *ExchangeMove) String() string {
	return "Exchanged letters: " + move.Letters
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
func (move *ExchangeMove) Apply(game *Game) bool {
	rack := game.PlayerToMove().Rack
	tiles := make([]*Tile, 0, RackSize)
	// First, remove the exchanged tiles from the player's Rack
	for _, letter := range move.Letters {
		tile, err := rack.GetTile(letter)
		if err != nil {
			// Should not happen!
			return false
		}
		if !rack.Remove(tile.Letter) {
			// Should not happen!
			return false
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
	return true
}

// Score is always 0 for an ExchangeMove
func (move *ExchangeMove) Score(state *GameState) int {
	return 0
}

// NewFinalMove returns a reference to a fresh FinalMove
func NewFinalMove(rackOpp string, multiplyFactor int) *FinalMove {
	return &FinalMove{OpponentRack: rackOpp, MultiplyFactor: multiplyFactor}
}

// String return a string description of the FinalMove
func (move *FinalMove) String() string {
	return "Rack " + move.OpponentRack
}

// IsValid always returns true for a FinalMove
func (move *FinalMove) IsValid(game *Game) bool {
	return true
}

// Apply always succeeds and returns true for a FinalMove
func (move *FinalMove) Apply(game *Game) bool {
	return true
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

type Axis struct {
	state             *GameState
	horizontal        bool
	rack              []rune
	rackString        string
	squares           [BoardSize]*Square
	crossCheckLetters [BoardSize][]rune
	isAnchor          [BoardSize]bool
}

func (gs *GameState) GenerateMoves() []Move {
	leftParts := FindLeftParts(gs.Dawg, gs.Rack.AsString())

	// results := make(chan []Move, BoardSize*2)
	resultsChan := make(chan []Move, BoardSize*2)

	// Start the 30 goroutines (columns and rows = 2 * BoardSize)
	// Horizontal rows
	for row := 0; row < BoardSize; row++ {
		go gs.GenerateMovesOnAxis(row, true, leftParts, resultsChan)
	}
	// Vertical columns
	for col := 0; col < BoardSize; col++ {
		go gs.GenerateMovesOnAxis(col, false, leftParts, resultsChan)
	}

	// Collect move candidates from all goroutines and
	// append them to the moves list
	moves := make([]Move, 0)
	for i := 0; i < BoardSize*2; i++ {
		moves = append(moves, <-resultsChan...)
	}

	return moves
}

// func (gs *GameState) GenerateMovesOnAxis(index int, horizontal bool, leftParts [][]*LeftPart, moves chan<- []Move) {
func (gs *GameState) GenerateMovesOnAxis(index int, horizontal bool, leftParts [][]*LeftPart, resultsChan chan<- []Move) {
	var axis Axis
	axis.Init(gs, index, horizontal)
	// Generate a list of moves and send it on the result channel
	resultsChan <- axis.GenerateMoves(len(gs.Rack.Tiles), leftParts)
}

func (axis *Axis) Init(state *GameState, index int, horizontal bool) {
	axis.state = state
	axis.horizontal = horizontal
	axis.rack = state.Rack.AsRunes()
	axis.rackString = state.Rack.AsString()
	board := state.Board
	// Build an array of pointers to the squares on this axis
	for i := 0; i < BoardSize; i++ {
		if horizontal {
			axis.squares[i] = board.GetSquare(Position{Row: index, Col: i})
		} else {
			axis.squares[i] = board.GetSquare(Position{Row: i, Col: index})
		}
	}
	// Mark all empty squares having at least one occupied
	// adjacent square as anchors
	for i := 0; i < BoardSize; i++ {
		sq := axis.squares[i]
		if sq.Tile != nil {
			// Already have a tile here: not an anchor and no
			// cross-check set needed
			continue
		}

		var isAnchor bool
		if board.Squares[BoardCenter][BoardCenter].Tile == nil {
			// If no tile has yet been placed on the board,
			// mark the center square of the center column as an anchor
			isAnchor = (index == BoardCenter) && (i == BoardCenter) && !horizontal
		} else {
			isAnchor = sq.IsAnchor(board)
		}

		if !isAnchor {
			// Empty square with no adjacent tiles: not an anchor,
			// and we can place any letter from the rack here
			axis.crossCheckLetters[i] = axis.rack
		} else {
			// This is an anchor square, i.e. an empty square with
			// at least one adjacent tile. Playable letters are the ones
			// that are in the rack and the crossCheck.
			// Note, however, that the cross-check set for it may be zero,
			// if no tile from the rack can be placed in it due to
			// cross-words.
			axis.isAnchor[i] = true

			var crossCheckLetters []rune
			if len(axis.rack) != 0 {
				playable := axis.CrossCheck(sq)
				for _, l := range playable {
					if !strings.ContainsRune(axis.rackString, l) {
						continue
					}

					crossCheckLetters = append(crossCheckLetters, l)
				}
			}

			axis.crossCheckLetters[i] = crossCheckLetters
		}
	}
}

func (axis *Axis) CrossCheck(sq *Square) []rune {
	// Check whether the cross word(s) limit the set of allowed
	// letters in this anchor square
	prev, after := axis.state.Board.CrossWordFragments(sq.Position, !axis.horizontal)
	if len(prev) == 0 && len(after) == 0 {
		// No cross word, so no cross check constraint
		return axis.rack
	}
	return axis.state.Dawg.CrossCheck(prev, after)
}

func (axis *Axis) GenerateMoves(lenRack int, leftParts [][]*LeftPart) []Move {
	moves := make([]Move, 0)
	lastAnchor := -1
	// Process the anchors, one by one, from left to right
	for i := 0; i < BoardSize; i++ {
		if !axis.IsAnchor(i) {
			continue
		}
		// This is an anchor
		if len(axis.crossCheckLetters[i]) > 0 {
			// A tile from the rack can actually be placed here:
			// count open squares to the anchor's left,
			// up to but not including the previous anchor, if any.
			// Open squares are squares that are empty and can
			// accept a tile from the rack.
			openCnt := 0
			left := i
			for left > 0 && left > (lastAnchor+1) && axis.IsOpen(left-1) {
				openCnt++
				left--
			}
			moves = append(moves,
				axis.genMovesFromAnchor(i, min(openCnt, lenRack-1), leftParts)...,
			)
		}
		lastAnchor = i
	}
	return moves
}

// genMovesFromAnchor returns the available moves that use the given square
// within the Axis as an anchor
func (axis *Axis) genMovesFromAnchor(anchor int, maxLeft int, leftParts [][]*LeftPart) []Move {
	// Are there letters before the anchor? If so, extend before the anchor
	if maxLeft == 0 && anchor > 0 && axis.squares[anchor-1].Tile != nil {
		return axis.extendBefore(anchor)
	}
	// No letters before the anchor, so just extend after
	return axis.extendAfter(anchor, maxLeft, leftParts)
}

// extendBefore finds all moves that have tiles before the anchor where
// we can place tiles after the anchor
func (axis *Axis) extendBefore(anchor int) []Move {
	dawg, board := axis.state.Dawg, axis.state.Board
	sq := axis.squares[anchor]

	var direction Direction
	if axis.horizontal {
		direction = DirectionLeft
	} else {
		direction = DirectionAbove
	}
	// Get the entire left part, as a list of Tiles
	fragment := board.TileFragment(sq.Position, direction)
	// The fragment list is backwards; convert it to a proper Prefix,
	// which is a list of runes
	left := make([]rune, len(fragment))
	for i, tile := range fragment {
		left[len(fragment)-1-i] = tile.Letter
	}
	// Do the DAWG navigation to find the left part
	var lfn LeftFindNavigator
	lfn.Init(left)
	dawg.NavigateResumable(&lfn)
	if lfn.state == nil {
		// No matching prefix found: there cannot be any
		// valid completions of the left part that is already
		// there.
		return nil
	}
	// We found a matching prefix in the dawg:
	// do an ExtendRight from that location, using the whole rack
	var ern ExtendRightNavigator
	ern.Init(axis, anchor, axis.rackString)
	dawg.Resume(&ern, lfn.state, string(left))
	return ern.moves
}

// extendAfter finds all moves where we can place tiles after the anchor
func (axis *Axis) extendAfter(anchor int, maxLeft int, leftParts [][]*LeftPart) []Move {
	// We are not completing an existing previous part
	// Extend tiles from the anchor and after
	dawg := axis.state.Dawg
	moves := make([]Move, 0)
	var ern ExtendRightNavigator
	ern.Init(axis, anchor, axis.rackString)
	dawg.Navigate(&ern)
	// Collect the moves found so far
	moves = append(moves, ern.moves...)

	// Follow this by an effort to permute left prefixes into the
	// open space to the left of the anchor square, if any
	for leftLen := 1; leftLen <= maxLeft; leftLen++ {
		// Try all left prefixes of length leftLen
		leftList := leftParts[leftLen-1]
		for _, leftPart := range leftList {
			var ern ExtendRightNavigator
			ern.Init(axis, anchor, leftPart.rack)
			dawg.Resume(&ern, leftPart.state, leftPart.matched)
			moves = append(moves, ern.moves...)
		}
	}

	// Return the accumulated move list
	return moves
}

func (axis *Axis) IsAnchor(index int) bool {
	return axis.isAnchor[index]
}

// IsOpen returns true if the given square within the Axis
// is open for a new Tile from the Rack
func (axis *Axis) IsOpen(index int) bool {
	return axis.squares[index].Tile == nil && len(axis.crossCheckLetters[index]) > 0
}

// Allows returns true if the given letter can be placed
// in the indexed square within the Axis, in compliance
// with the cross checks
func (axis *Axis) Allows(index int, letter rune) bool {
	if axis == nil || axis.squares[index].Tile != nil {
		// We already have a tile in this square
		return false
	}

	return slices.Contains(axis.crossCheckLetters[index], letter)
}

func min(i1, i2 int) int {
	if i1 <= i2 {
		return i1
	}
	return i2
}
