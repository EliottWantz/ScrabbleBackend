package scrabble

import (
	"strings"

	"golang.org/x/exp/slices"
)

type Axis struct {
	state             *GameState
	horizontal        bool
	rack              []rune
	rackString        string
	squares           [BoardSize]*Square
	crossCheckLetters [BoardSize][]rune
	isAnchor          [BoardSize]bool
}

func (axis *Axis) Init(state *GameState, index int, horizontal bool) {
	axis.state = state
	axis.horizontal = horizontal
	axis.rack = state.Rack.AsRunes()
	axis.rackString = state.Rack.AsString()
	b := state.Board
	// Build an array of pointers to the squares on this axis
	for i := 0; i < BoardSize; i++ {
		if horizontal {
			axis.squares[i] = b.GetSquare(Position{Row: index, Col: i})
		} else {
			axis.squares[i] = b.GetSquare(Position{Row: i, Col: index})
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
		if b.Squares[BoardCenter][BoardCenter].Tile == nil {
			// If no tile has yet been placed on the board,
			// mark the center square of the center column as an anchor
			isAnchor = (index == BoardCenter) && (i == BoardCenter) && !horizontal
		} else {
			isAnchor = sq.IsAnchor(b)
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
	dawg, b := axis.state.Dawg, axis.state.Board
	sq := axis.squares[anchor]

	var direction Direction
	if axis.horizontal {
		direction = DirectionLeft
	} else {
		direction = DirectionAbove
	}
	// Get the entire left part, as a list of Tiles
	fragment := b.TileFragment(sq.Position, direction)
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
