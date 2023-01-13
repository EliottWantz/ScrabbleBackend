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

func (a *Axis) Init(gs *GameState, index int, horizontal bool) {
	a.state = gs
	a.horizontal = horizontal
	a.rack = gs.Rack.AsRunes()
	a.rackString = gs.Rack.AsString()
	b := gs.Board
	// Build an array of pointers to the squares on this axis
	for i := 0; i < BoardSize; i++ {
		if horizontal {
			a.squares[i] = b.GetSquare(Position{Row: index, Col: i})
		} else {
			a.squares[i] = b.GetSquare(Position{Row: i, Col: index})
		}
	}
	// Mark all empty squares having at least one occupied
	// adjacent square as anchors
	for i := 0; i < BoardSize; i++ {
		s := a.squares[i]
		if s.Tile != nil {
			// Already have a tile here: not an anchor and no
			// cross-check set needed
			continue
		}

		var isAnchor bool
		if b.Squares[BoardCenter][BoardCenter].Tile == nil {
			// If no tile has yet been placed on the board,
			// mark the center square of the center column as an anchor
			isAnchor = (index == BoardCenter) && (i == BoardCenter)
		} else {
			isAnchor = s.IsAnchor(b)
		}

		if !isAnchor {
			if strings.ContainsRune(a.rackString, '*') {
				a.crossCheckLetters[i] = []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z'}
				continue
			}
			// Empty square with no adjacent tiles: not an anchor,
			// and we can place any letter from the rack here
			a.crossCheckLetters[i] = a.rack
		} else {
			// This is an anchor square, i.e. an empty square with
			// at least one adjacent tile. Playable letters are the ones
			// that are in the rack and the crossCheck.
			// Note, however, that the cross-check set for it may be zero,
			// if no tile from the rack can be placed in it due to
			// cross-words.
			a.isAnchor[i] = true

			var crossCheckLetters []rune
			if len(a.rack) != 0 {
				playable := a.CrossCheck(s)
				if strings.ContainsRune(a.rackString, '*') {
					crossCheckLetters = playable
				} else {
					for _, l := range playable {
						if !strings.ContainsRune(a.rackString, l) {
							continue
						}

						crossCheckLetters = append(crossCheckLetters, l)
					}
				}
			}

			a.crossCheckLetters[i] = crossCheckLetters
		}
	}
}

func (a *Axis) CrossCheck(s *Square) []rune {
	// Check whether the cross word(s) limit the set of allowed
	// letters in this anchor square
	prev, after := a.state.Board.CrossWordFragments(s.Position, !a.horizontal)
	if len(prev) == 0 && len(after) == 0 {
		// No cross word, so no cross check constraint
		return a.rack
	}
	return a.state.DAWG.CrossCheck(prev, after)
}

func (a *Axis) GenerateMoves(leftParts [][]*LeftPart) []Move {
	moves := make([]Move, 0)
	lastAnchor := -1
	// Process the anchors, one by one, from left to right
	for i := 0; i < BoardSize; i++ {
		if !a.IsAnchor(i) {
			continue
		}
		// This is an anchor
		if len(a.crossCheckLetters[i]) > 0 {
			// A tile from the rack can actually be placed here:
			// count open squares to the anchor's left,
			// up to but not including the previous anchor, if any.
			// Open squares are squares that are empty and can
			// accept a tile from the rack.
			openCnt := 0
			left := i
			for left > 0 && left > (lastAnchor+1) && a.IsOpen(left-1) {
				openCnt++
				left--
			}
			moves = append(moves,
				a.genMovesFromAnchor(i, min(openCnt, len(a.state.Rack.Tiles)-1), leftParts)...,
			)
		}
		lastAnchor = i
	}
	return moves
}

// genMovesFromAnchor returns the available moves that use the given square
// within the Axis as an anchor
func (a *Axis) genMovesFromAnchor(anchor int, maxLeft int, leftParts [][]*LeftPart) []Move {
	// Are there letters before the anchor? If so, extend before the anchor
	if maxLeft == 0 && anchor > 0 && a.squares[anchor-1].Tile != nil {
		return a.extendBefore(anchor)
	}
	// No letters before the anchor, so just extend after
	return a.extendAfter(anchor, maxLeft, leftParts)
}

// extendBefore finds all moves that have tiles before the anchor where
// we can place tiles after the anchor
func (a *Axis) extendBefore(anchor int) []Move {
	DAWG, b := a.state.DAWG, a.state.Board
	s := a.squares[anchor]

	var dir Direction
	if a.horizontal {
		dir = DirectionLeft
	} else {
		dir = DirectionAbove
	}
	// Get the entire left part, as a list of Tiles
	fragment := b.TileFragment(s.Position, dir)
	// The fragment list is backwards; convert it to a proper Prefix,
	// which is a list of runes
	left := make([]rune, len(fragment))
	for i, tile := range fragment {
		left[len(fragment)-1-i] = tile.Letter
	}
	// Do the DAWG navigation to find the left part
	var ebn ExtendBeforeNavigator
	ebn.Init(left)
	DAWG.NavigateResumable(&ebn)
	if ebn.navState == nil {
		// No matching prefix found: there cannot be any
		// valid completions of the left part that is already
		// there.
		return nil
	}
	// We found a matching prefix in the dawg:
	// do an ExtendRight from that location, using the whole rack
	var ean ExtendAfterNavigator
	ean.Init(a, anchor, a.rackString)
	DAWG.Resume(&ean, ebn.navState, string(left))
	return ean.moves
}

// extendAfter finds all moves where we can place tiles after the anchor
func (a *Axis) extendAfter(anchor int, maxLeft int, leftParts [][]*LeftPart) []Move {
	// We are not completing an existing previous part
	// Extend tiles from the anchor and after
	DAWG := a.state.DAWG
	moves := make([]Move, 0)

	var ean ExtendAfterNavigator
	ean.Init(a, anchor, a.rackString)
	DAWG.Navigate(&ean)
	moves = append(moves, ean.moves...)

	// Follow this by an effort to permute left prefixes into the
	// open space to the left of the anchor square, if any
	for leftLen := 1; leftLen <= maxLeft; leftLen++ {
		// Try all left prefixes of length leftLen
		leftList := leftParts[leftLen-1]
		for _, leftPart := range leftList {
			var ean ExtendAfterNavigator
			ean.Init(a, anchor, leftPart.rack)
			DAWG.Resume(&ean, leftPart.navState, leftPart.matched)
			moves = append(moves, ean.moves...)
		}
	}

	return moves
}

func (a *Axis) IsAnchor(index int) bool {
	return a.isAnchor[index]
}

// IsOpen returns true if the given square within the Axis
// is open for a new Tile from the Rack
func (a *Axis) IsOpen(index int) bool {
	return a.squares[index].Tile == nil && len(a.crossCheckLetters[index]) > 0
}

// Allows returns true if the given letter can be placed
// in the indexed square within the Axis, in compliance
// with the cross checks
func (a *Axis) Allows(index int, letter rune) bool {
	if a == nil || a.squares[index].Tile != nil {
		return false
	}

	return slices.Contains(a.crossCheckLetters[index], letter)
}

func min(i1, i2 int) int {
	if i1 <= i2 {
		return i1
	}
	return i2
}
