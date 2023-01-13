package scrabble

import (
	"strings"
)

type Match int

// Matching constants
const (
	MatchNo        Match = 1
	MatchBoardTile Match = 2
	MacthRackTile  Match = 3
)

// Make sure the navigator structs implements the Navigator interface
var (
	_ Navigator = (*ExtendBeforeNavigator)(nil)
	_ Navigator = (*ExtendAfterNavigator)(nil)
	_ Navigator = (*LeftPermutationNavigator)(nil)
	_ Navigator = (*MatchNavigator)(nil)
	_ Navigator = (*FindNavigator)(nil)
)

// Navigator is an interface that describes behaviors that control the
// navigation of a Dawg
type Navigator interface {
	IsAccepting() bool
	Accepts(rune) bool
	Accept(matched string, isWord bool, ns *navState)
	PushEdge(rune) bool
	PopEdge() bool
}

type ExtendBeforeNavigator struct {
	prefix    []rune
	lenPrefix int
	index     int
	// navState is the result of the ExtendBeforeNavigator,
	// which is used to continue navigation after a previous part
	// has been found on the board
	navState *navState
}

// ExtendAfterNavigator implements the core of the Appel-Jacobson
// algorithm. It proceeds along an Axis, covering empty Squares with
// Tiles from the Rack while obeying constraints from the Dawg and
// the cross-check sets. As final nodes in the Dawg are encountered,
// valid tile moves are generated and saved.
type ExtendAfterNavigator struct {
	axis           *Axis
	anchor         int
	index          int
	rack           string
	stack          []eanItem
	lastCheck      Match
	wildcardInRack bool
	moves          []Move
}

type eanItem struct {
	rack           string
	index          int
	wildcardInRack bool
}

type LeftPermutationNavigator struct {
	rack      string
	stack     []leftPermItem
	maxLeft   int
	leftParts [][]*LeftPart
	index     int
}

type leftPermItem struct {
	rack  string
	index int
}

type MatchNavigator struct {
	pattern    []rune
	lenP       int
	index      int
	chMatch    rune
	isWildcard bool
	stack      []matchItem
	results    []string
}

type matchItem struct {
	index      int
	chMatch    rune
	isWildcard bool
}

type FindNavigator struct {
	word    []rune
	lenWord int
	index   int
	found   bool
}

type Navigation struct {
	DAWG        *DAWG
	navigator   Navigator
	isResumable bool
}

type LeftPart struct {
	matched  string
	rack     string
	navState *navState
}

type navState struct {
	prefix   rune
	nextNode *Node
}

func (ebn *ExtendBeforeNavigator) Init(prefix []rune) {
	ebn.prefix = prefix
	ebn.lenPrefix = len(prefix)
}

// PushEdge determines whether the navigation should proceed into
// an edge having c as its first letter
func (ebn *ExtendBeforeNavigator) PushEdge(c rune) bool {
	// If the edge matches our place in the sought word, go for it
	return ebn.prefix[ebn.index] == c
}

// PopEdge returns false if there is no need to visit other edges
// after this one has been traversed
func (ebn *ExtendBeforeNavigator) PopEdge() bool {
	// There can only be one correct outgoing edge for the
	// Find function, so we return false to prevent other edges
	// from being tried
	return false
}

// IsAccepting returns false if the navigator should not expect more
// characters
func (ebn *ExtendBeforeNavigator) IsAccepting() bool {
	return ebn.index < ebn.lenPrefix
}

// Accepts returns true if the navigator should accept and 'eat' the
// given character
func (ebn *ExtendBeforeNavigator) Accepts(c rune) bool {
	// For the ExtendBeforeNavigator, we never enter an edge unless
	// we have the correct character, so we simply advance
	// the index and return true
	ebn.index++
	return true
}

// Accept is called to inform the navigator of a match and
// whether it is a isWord word
func (ebn *ExtendBeforeNavigator) Accept(matched string, isWord bool, ns *navState) {
	if ebn.index == ebn.lenPrefix {
		// Found the whole left part; save its position navState
		ebn.navState = ns
	}
}

// Init initializes a fresh ExtendAfterNavigator for an axis, starting
// from the given anchor, using the indicated rack
func (ean *ExtendAfterNavigator) Init(axis *Axis, anchor int, rack string) {
	ean.axis = axis
	ean.anchor = anchor
	ean.index = anchor
	ean.rack = rack
	ean.wildcardInRack = strings.ContainsRune(rack, '*')
	ean.stack = make([]eanItem, 0, RackSize)
	ean.moves = make([]Move, 0)
}

func (ean *ExtendAfterNavigator) check(letter rune) Match {
	tileAtSq := ean.axis.squares[ean.index].Tile
	if tileAtSq != nil {
		// There is a tile in the square: must match it exactly
		if letter == tileAtSq.Letter {
			// Matches, from the board
			return MatchBoardTile
		}
		// Doesn't match the tile that is already there
		return MatchNo
	}
	// Does the current rack allow this letter?
	if !ean.wildcardInRack && !strings.ContainsRune(ean.rack, letter) {
		// No, it doesn't
		return MatchNo
	}
	// Finally, test the cross-checks
	if ean.axis.Allows(ean.index, letter) {
		// The tile successfully completes any cross-words
		/*
			// DEBUG: verify that the cross-checks hold
			sq := ean.axis.squares[ean.index]
			left, right := ean.axis.state.Board.CrossWords(sq.Row, sq.Col, !ean.axis.horizontal)
			if left != "" || right != "" {
				word := left + string(letter) + right
				if !ean.axis.state.Dawg.Find(word) {
					panic("Cross-check violation!")
				}
			}
		*/
		return MacthRackTile
	}
	return MatchNo
}

// PushEdge determines whether the navigation should proceed into
// an edge having c as its first letter
func (ean *ExtendAfterNavigator) PushEdge(letter rune) bool {
	ean.lastCheck = ean.check(letter)
	if ean.lastCheck == MatchNo {
		// No way that this letter can be laid down here
		return false
	}
	// Match: save our rack and our index and move into the edge
	ean.stack = append(ean.stack, eanItem{ean.rack, ean.index, ean.wildcardInRack})
	return true
}

// PopEdge returns false if there is no need to visit other edges
// after this one has been traversed
func (ean *ExtendAfterNavigator) PopEdge() bool {
	// Pop the previous rack and index from the stack
	last := len(ean.stack) - 1
	sp := &ean.stack[last]
	ean.rack, ean.index, ean.wildcardInRack = sp.rack, sp.index, sp.wildcardInRack
	ean.stack = ean.stack[0:last]
	// We need to visit all outgoing edges, so return true
	return true
}

// IsAccepting returns false if the navigator should not expect more
// characters
func (ean *ExtendAfterNavigator) IsAccepting() bool {
	if ean.index >= BoardSize {
		// Gone off the board edge
		return false
	}
	// Otherwise, continue while we have something on the rack
	// or we're at an occupied square
	return len(ean.rack) > 0 || ean.axis.squares[ean.index].Tile != nil
}

// Accepts returns true if the navigator should accept and 'eat' the
// given character
func (ean *ExtendAfterNavigator) Accepts(letter rune) bool {
	// We are on the anchor square or to its right
	match := ean.lastCheck
	if match == 0 {
		// No cached check available from PushEdge
		match = ean.check(letter)
	}
	ean.lastCheck = 0
	if match == MatchNo {
		// No fit anymore: we're done with this edge
		return false
	}
	// This letter is OK: accept it and remove from the rack if
	// it came from there
	ean.index++
	if match == MacthRackTile {
		if strings.ContainsRune(ean.rack, letter) {
			// Used a normal tile
			ean.rack = strings.Replace(ean.rack, string(letter), "", 1)
		} else {
			// Used a blank tile
			ean.rack = strings.Replace(ean.rack, "*", "", 1)
		}
		ean.wildcardInRack = strings.ContainsRune(ean.rack, '*')
	}
	return true
}

// Accept is called to inform the navigator of a match and
// whether it is a isWord word
func (ean *ExtendAfterNavigator) Accept(matched string, isWord bool, ns *navState) {
	if ns != nil {
		panic("ExtendAfterNavigator should not be resumable")
	}
	if !isWord ||
		(ean.index < BoardSize && ean.axis.squares[ean.index].Tile != nil) {
		// Not a complete word, or ends on an occupied square:
		// not a legal tile move
		return
	}
	runes := []rune(matched)
	if len(runes) < 2 {
		// Less than 2 letters long: not a legal tile move
		return
	}
	// Legal move found: make a TileMove object for it and add to
	// the move list
	covers := make(Covers)
	// Calculate the starting index within the axis
	start := ean.index - len(runes)
	// The original rack
	rack := ean.axis.rackString
	for i, actualLetter := range runes {
		sq := ean.axis.squares[start+i]
		if sq.Tile == nil {
			letter := actualLetter
			if strings.ContainsRune(rack, actualLetter) {
				rack = strings.Replace(rack, string(actualLetter), "", 1)
			} else {
				// Must be using a blank tile
				letter = '*'
				rack = strings.Replace(rack, "*", "", 1)
			}
			covers[sq.Position] = Cover{Letter: letter, Actual: actualLetter}
		}
	}
	// No need to validate robot-generated tile moves
	tileMove := NewUncheckedTileMove(ean.axis.state.Board, covers)
	ean.moves = append(ean.moves, tileMove)
}

// Init initializes a fresh LeftPermutationNavigator using the given rack
func (lpn *LeftPermutationNavigator) Init(rack string) {
	lpn.rack = rack
	// One tile from the rack will be put on the anchor square;
	// the rest is available to be played to the left of the anchor.
	// We thus find all permutations involving all rack tiles except
	// one.
	lenRack := len(rack)
	if lenRack <= 1 {
		// No left permutation possible
		lpn.maxLeft = 0
	} else {
		lpn.maxLeft = lenRack - 1
	}
	lpn.stack = make([]leftPermItem, 0)
	lpn.leftParts = make([][]*LeftPart, lpn.maxLeft)
	for i := 0; i < lpn.maxLeft; i++ {
		lpn.leftParts[i] = make([]*LeftPart, 0)
	}
}

// IsAccepting returns false if the navigator should not expect more
// characters
func (lpn *LeftPermutationNavigator) IsAccepting() bool {
	return lpn.index < lpn.maxLeft
}

// Accepts returns true if the navigator should accept and 'eat' the
// given character
func (lpn *LeftPermutationNavigator) Accepts(char rune) bool {
	exactMatch := strings.ContainsRune(lpn.rack, char)
	if !exactMatch && !strings.ContainsRune(lpn.rack, '*') {
		return false
	}
	lpn.index++
	if exactMatch {
		lpn.rack = strings.Replace(lpn.rack, string(char), "", 1)
	} else {
		lpn.rack = strings.Replace(lpn.rack, "*", "", 1)
	}
	return true
}

// Accept is called to inform the navigator of a match and
// whether it is a isWord word
func (lpn *LeftPermutationNavigator) Accept(matched string, isWord bool, ns *navState) {
	ix := len([]rune(matched)) - 1
	lpn.leftParts[ix] = append(lpn.leftParts[ix],
		&LeftPart{matched: matched, rack: lpn.rack, navState: ns},
	)
}

func (lpn *LeftPermutationNavigator) PushEdge(char rune) bool {
	if !strings.ContainsRune(lpn.rack, char) && !strings.ContainsRune(lpn.rack, '*') {
		return false
	}
	lpn.stack = append(lpn.stack, leftPermItem{lpn.rack, lpn.index})
	return true
}

// PopEdge returns false if there is no need to visit other edges
// after this one has been traversed
func (lpn *LeftPermutationNavigator) PopEdge() bool {
	// Pop the previous rack and index from the stack
	last := len(lpn.stack) - 1
	lpn.rack, lpn.index = lpn.stack[last].rack, lpn.stack[last].index
	lpn.stack = lpn.stack[0:last]
	return true
}

// Init initializes a MatchNavigator with the word to search for
func (mn *MatchNavigator) Init(pattern string) {
	// Convert the word to a list of runes
	mn.pattern = []rune(pattern)
	mn.lenP = len(mn.pattern)
	mn.chMatch = mn.pattern[0]
	mn.isWildcard = mn.chMatch == '*'
	mn.stack = make([]matchItem, 0, RackSize)
	mn.results = make([]string, 0)
}

// PushEdge determines whether the navigation should proceed into
// an edge having c as its first letter
func (mn *MatchNavigator) PushEdge(c rune) bool {
	if c != mn.chMatch && !mn.isWildcard {
		return false
	}
	mn.stack = append(mn.stack, matchItem{mn.index, mn.chMatch, mn.isWildcard})
	return true
}

// PopEdge returns false if there is no need to visit other edges
// after this one has been traversed
func (mn *MatchNavigator) PopEdge() bool {
	last := len(mn.stack) - 1
	mt := &mn.stack[last]
	mn.index, mn.chMatch, mn.isWildcard = mt.index, mt.chMatch, mt.isWildcard
	mn.stack = mn.stack[0:last]
	return mn.isWildcard
}

// IsAccepting returns false if the navigator should not expect more
// characters
func (mn *MatchNavigator) IsAccepting() bool {
	return mn.index < mn.lenP
}

// Accepts returns true if the navigator should accept and 'eat' the
// given character
func (mn *MatchNavigator) Accepts(c rune) bool {
	if c != mn.chMatch && !mn.isWildcard {
		// Not a correct next character in the word
		return false
	}
	// This is a correct character: advance our index
	mn.index++
	if mn.index < mn.lenP {
		mn.chMatch = mn.pattern[mn.index]
		mn.isWildcard = mn.chMatch == '*'
	}
	return true
}

// Accept is called to inform the navigator of a match and
// whether it is a isWord word
func (mn *MatchNavigator) Accept(matched string, isWord bool, ns *navState) {
	if isWord && mn.index == mn.lenP {
		// Entire pattern match
		mn.results = append(mn.results, matched)
	}
}

// Init initializes a FindNavigator with the word to search for
func (fn *FindNavigator) Init(word string) {
	fn.word = []rune(word)
	fn.lenWord = len(fn.word)
}

// PushEdge determines whether the navigation should proceed into
// an edge having c as its first letter
func (fn *FindNavigator) PushEdge(c rune) bool {
	// If the edge matches our place in the sought word, go for it
	return fn.word[fn.index] == c
}

// PopEdge returns false if there is no need to visit other edges
// after this one has been traversed
func (fn *FindNavigator) PopEdge() bool {
	// There can only be one correct outgoing edge for the
	// Find function, so we return false to prevent other edges
	// from being tried
	return false
}

// IsAccepting returns false if the navigator should not expect more
// characters
func (fn *FindNavigator) IsAccepting() bool {
	return fn.index < fn.lenWord
}

// Accepts returns true if the navigator should accept and 'eat' the
// given character
func (fn *FindNavigator) Accepts(c rune) bool {
	// For the FindNavigator, we never enter an edge unless
	// we have the correct character, so we simply advance
	// the index and return true
	fn.index++
	return true
}

// Accept is called to inform the navigator of a match and
// whether it is a isWord word
func (fn *FindNavigator) Accept(matched string, isWord bool, ns *navState) {
	if isWord && fn.index == fn.lenWord {
		// This is a whole word (isWord=true) and matches our
		// length, so that's it
		fn.found = true
	}
}

// Go starts a navigation on the underlying Dawg using the given
// Navigator
func (nav *Navigation) Go(d *DAWG, navigator Navigator) {
	nav.DAWG = d
	nav.navigator = navigator
	if navigator.IsAccepting() {
		nav.FromNode(d.Root, "")
	}
}

// FromNode continues a navigation from a node in the Dawg,
// enumerating through outgoing edges until the navigator is
// satisfied
func (nav *Navigation) FromNode(n *Node, matched string) {
	iter := nav.DAWG.iterNodeCache.iterNode(n)
	for i := 0; i < len(iter); i++ {
		ns := &iter[i]
		if nav.navigator.PushEdge(ns.prefix) {
			// The navigator wants us to enter this edge
			nav.FromEdge(ns, matched)
			if !nav.navigator.PopEdge() {
				// The navigator doesn't want to visit
				// other edges, so we're done with this node
				break
			}
		}
	}
}

// FromEdge navigates along an edge in the Dawg. An edge
// consists of a prefix string, which may be longer than
// one letter.
func (nav *Navigation) FromEdge(ns *navState, matched string) {
	if ns.prefix == NoPrefix {
		return
	}
	navigator := nav.navigator
	if !navigator.Accepts(ns.prefix) {
		return
	}
	// The navigator wants this prefix letter:
	// add it to the matched prefix and find out whether
	// it is now an entire valid word
	matched += string(ns.prefix)
	isWord := false
	if ns.nextNode == nil || ns.nextNode.IsWord {
		isWord = true
	}
	// Notify the navigator of the match
	if nav.isResumable {
		// We want the full navigation state to be passed to navigator.Accept()
		navigator.Accept(
			matched,
			isWord,
			// Create a navState that would resume the navigation at our
			// current location within the prefix, with the same nextNode
			&navState{prefix: ns.prefix, nextNode: ns.nextNode},
		)
	} else {
		// No need to pass the full state
		navigator.Accept(matched, isWord, nil)
	}
	if ns.nextNode != nil && navigator.IsAccepting() {
		// Completed a whole prefix and still the navigator
		// has appetite: continue to the following node
		nav.FromNode(ns.nextNode, matched)
	}
}

// Resume continues a navigation on the underlying Dawg
// using the given Navigator, from a previously saved navigation
// state
func (nav *Navigation) Resume(d *DAWG, navigator Navigator, ns *navState, matched string) {
	nav.DAWG = d
	nav.navigator = navigator
	if navigator.IsAccepting() {
		nav.FromEdge(ns, matched)
	}
}
