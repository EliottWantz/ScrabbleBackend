package scrabble

import (
	"strings"
)

type Match int

// Matching constants
const (
	MatchNo        Match = 1
	MatchBoardTile       = 2
	MacthRackTile        = 3
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
	Accept(matched string, final bool, state *navState)
	PushEdge(rune) bool
	PopEdge() bool
	Done()
}

// ExtendBeforeNavigator is similar to FindNavigator, but instead of returning
// only a bool result, it returns the full navigation state as it is when
// the requested word prefix is found. This makes it possible to continue
// the navigation later with further constraints.
type ExtendBeforeNavigator struct {
	prefix    []rune
	lenPrefix int
	index     int
	// state is the result of the ExtendBeforeNavigator,
	// which is used to continue navigation after a left part
	// has been found on the board
	state *navState
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

// LeftPermutationNavigator finds all left parts of words that are
// possible with a particular rack, and accumulates them by length.
// This is done once at the start of move generation.
type LeftPermutationNavigator struct {
	rack      string
	stack     []leftPermItem
	maxLeft   int
	leftParts [][]*LeftPart
	index     int
}

// MatchNavigator stores the state for a pattern matching
// navigation of a DAWG
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

// FindNavigator stores the state for a plain word search in the DAWG
type FindNavigator struct {
	word    []rune
	lenWord int
	index   int
	found   bool
}

// Navigation contains the state of a single navigation that is
// underway within a Dawg
type Navigation struct {
	dawg        *DAWG
	navigator   Navigator
	isResumable bool
}

type leftPermItem struct {
	rack  string
	index int
}

// LeftPart stores the navigation state after matching a particular
// left part within the DAWG, so we can resume navigation from that
// point to complete an anchor square followed by a right part
type LeftPart struct {
	matched string
	rack    string
	state   *navState
}

// navState holds a navigation state, i.e. an edge where a prefix
// leads to a nextNode
type navState struct {
	prefix   rune
	nextNode *Node
}

// Init initializes a ExtendBeforeNavigator with the word to search for
func (fbn *ExtendBeforeNavigator) Init(prefix []rune) {
	fbn.prefix = prefix
	fbn.lenPrefix = len(prefix)
}

// PushEdge determines whether the navigation should proceed into
// an edge having chr as its first letter
func (fbn *ExtendBeforeNavigator) PushEdge(chr rune) bool {
	// If the edge matches our place in the sought word, go for it
	return fbn.prefix[fbn.index] == chr
}

// PopEdge returns false if there is no need to visit other edges
// after this one has been traversed
func (fbn *ExtendBeforeNavigator) PopEdge() bool {
	// There can only be one correct outgoing edge for the
	// Find function, so we return false to prevent other edges
	// from being tried
	return false
}

// Done is called when the navigation is complete
func (fbn *ExtendBeforeNavigator) Done() {
}

// IsAccepting returns false if the navigator should not expect more
// characters
func (fbn *ExtendBeforeNavigator) IsAccepting() bool {
	return fbn.index < fbn.lenPrefix
}

// Accepts returns true if the navigator should accept and 'eat' the
// given character
func (fbn *ExtendBeforeNavigator) Accepts(chr rune) bool {
	// For the ExtendBeforeNavigator, we never enter an edge unless
	// we have the correct character, so we simply advance
	// the index and return true
	fbn.index++
	return true
}

// Accept is called to inform the navigator of a match and
// whether it is a final word
func (fbn *ExtendBeforeNavigator) Accept(matched string, final bool, state *navState) {
	if fbn.index == fbn.lenPrefix {
		// Found the whole left part; save its position (state)
		fbn.state = state
	}
}

// Init initializes a fresh ExtendAfterNavigator for an axis, starting
// from the given anchor, using the indicated rack
func (ern *ExtendAfterNavigator) Init(axis *Axis, anchor int, rack string) {
	ern.axis = axis
	ern.anchor = anchor
	ern.index = anchor
	ern.rack = rack
	ern.wildcardInRack = strings.ContainsRune(rack, '*')
	ern.stack = make([]eanItem, 0, RackSize)
	ern.moves = make([]Move, 0)
}

func (ern *ExtendAfterNavigator) check(letter rune) Match {
	tileAtSq := ern.axis.squares[ern.index].Tile
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
	if !ern.wildcardInRack && !strings.ContainsRune(ern.rack, letter) {
		// No, it doesn't
		return MatchNo
	}
	// Finally, test the cross-checks
	if ern.axis.Allows(ern.index, letter) {
		// The tile successfully completes any cross-words
		/*
			// DEBUG: verify that the cross-checks hold
			sq := ern.axis.squares[ern.index]
			left, right := ern.axis.state.Board.CrossWords(sq.Row, sq.Col, !ern.axis.horizontal)
			if left != "" || right != "" {
				word := left + string(letter) + right
				if !ern.axis.state.Dawg.Find(word) {
					panic("Cross-check violation!")
				}
			}
		*/
		return MacthRackTile
	}
	return MatchNo
}

// PushEdge determines whether the navigation should proceed into
// an edge having chr as its first letter
func (ern *ExtendAfterNavigator) PushEdge(letter rune) bool {
	ern.lastCheck = ern.check(letter)
	if ern.lastCheck == MatchNo {
		// No way that this letter can be laid down here
		return false
	}
	// Match: save our rack and our index and move into the edge
	ern.stack = append(ern.stack, eanItem{ern.rack, ern.index, ern.wildcardInRack})
	return true
}

// PopEdge returns false if there is no need to visit other edges
// after this one has been traversed
func (ern *ExtendAfterNavigator) PopEdge() bool {
	// Pop the previous rack and index from the stack
	last := len(ern.stack) - 1
	sp := &ern.stack[last]
	ern.rack, ern.index, ern.wildcardInRack = sp.rack, sp.index, sp.wildcardInRack
	ern.stack = ern.stack[0:last]
	// We need to visit all outgoing edges, so return true
	return true
}

// Done is called when the navigation is complete
func (ern *ExtendAfterNavigator) Done() {
}

// IsAccepting returns false if the navigator should not expect more
// characters
func (ern *ExtendAfterNavigator) IsAccepting() bool {
	if ern.index >= BoardSize {
		// Gone off the board edge
		return false
	}
	// Otherwise, continue while we have something on the rack
	// or we're at an occupied square
	return len(ern.rack) > 0 || ern.axis.squares[ern.index] != nil
}

// Accepts returns true if the navigator should accept and 'eat' the
// given character
func (ern *ExtendAfterNavigator) Accepts(letter rune) bool {
	// We are on the anchor square or to its right
	match := ern.lastCheck
	if match == 0 {
		// No cached check available from PushEdge
		match = ern.check(letter)
	}
	ern.lastCheck = 0
	if match == MatchNo {
		// No fit anymore: we're done with this edge
		return false
	}
	// This letter is OK: accept it and remove from the rack if
	// it came from there
	ern.index++
	if match == MacthRackTile {
		if strings.ContainsRune(ern.rack, letter) {
			// Used a normal tile
			ern.rack = strings.Replace(ern.rack, string(letter), "", 1)
		} else {
			// Used a blank tile
			ern.rack = strings.Replace(ern.rack, "*", "", 1)
		}
		ern.wildcardInRack = strings.ContainsRune(ern.rack, '*')
	}
	return true
}

// Accept is called to inform the navigator of a match and
// whether it is a final word
func (ern *ExtendAfterNavigator) Accept(matched string, final bool, state *navState) {
	if state != nil {
		panic("ExtendAfterNavigator should not be resumable")
	}
	if !final ||
		(ern.index < BoardSize && ern.axis.squares[ern.index].Tile != nil) {
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
	start := ern.index - len(runes)
	// The original rack
	rack := ern.axis.rackString
	for i, meaning := range runes {
		sq := ern.axis.squares[start+i]
		if sq.Tile == nil {
			letter := meaning
			if strings.ContainsRune(rack, meaning) {
				rack = strings.Replace(rack, string(meaning), "", 1)
			} else {
				// Must be using a blank tile
				letter = '*'
				rack = strings.Replace(rack, "*", "", 1)
			}
			covers[sq.Position] = letter
		}
	}
	// No need to validate robot-generated tile moves
	tileMove := NewUncheckedTileMove(ern.axis.state.Board, covers)
	ern.moves = append(ern.moves, tileMove)
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
// whether it is a final word
func (lpn *LeftPermutationNavigator) Accept(matched string, final bool, state *navState) {
	ix := len([]rune(matched)) - 1
	lpn.leftParts[ix] = append(lpn.leftParts[ix],
		&LeftPart{matched: matched, rack: lpn.rack, state: state},
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

// Done is called when the navigation is complete
func (lpn *LeftPermutationNavigator) Done() {
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
// an edge having chr as its first letter
func (mn *MatchNavigator) PushEdge(chr rune) bool {
	if chr != mn.chMatch && !mn.isWildcard {
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

// Done is called when the navigation is complete
func (mn *MatchNavigator) Done() {
}

// IsAccepting returns false if the navigator should not expect more
// characters
func (mn *MatchNavigator) IsAccepting() bool {
	return mn.index < mn.lenP
}

// Accepts returns true if the navigator should accept and 'eat' the
// given character
func (mn *MatchNavigator) Accepts(chr rune) bool {
	if chr != mn.chMatch && !mn.isWildcard {
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
// whether it is a final word
func (mn *MatchNavigator) Accept(matched string, final bool, state *navState) {
	if final && mn.index == mn.lenP {
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
// an edge having chr as its first letter
func (fn *FindNavigator) PushEdge(chr rune) bool {
	// If the edge matches our place in the sought word, go for it
	return fn.word[fn.index] == chr
}

// PopEdge returns false if there is no need to visit other edges
// after this one has been traversed
func (fn *FindNavigator) PopEdge() bool {
	// There can only be one correct outgoing edge for the
	// Find function, so we return false to prevent other edges
	// from being tried
	return false
}

// Done is called when the navigation is complete
func (fn *FindNavigator) Done() {
}

// IsAccepting returns false if the navigator should not expect more
// characters
func (fn *FindNavigator) IsAccepting() bool {
	return fn.index < fn.lenWord
}

// Accepts returns true if the navigator should accept and 'eat' the
// given character
func (fn *FindNavigator) Accepts(chr rune) bool {
	// For the FindNavigator, we never enter an edge unless
	// we have the correct character, so we simply advance
	// the index and return true
	fn.index++
	return true
}

// Accept is called to inform the navigator of a match and
// whether it is a final word
func (fn *FindNavigator) Accept(matched string, final bool, state *navState) {
	if final && fn.index == fn.lenWord {
		// This is a whole word (final=true) and matches our
		// length, so that's it
		fn.found = true
	}
}

// Go starts a navigation on the underlying Dawg using the given
// Navigator
func (nav *Navigation) Go(dawg *DAWG, navigator Navigator) {
	nav.dawg = dawg
	nav.navigator = navigator
	if navigator.IsAccepting() {
		// Leave our home harbor and set sail for the open seas
		nav.FromNode(dawg.Root, "")
	}
	navigator.Done()
}

// FromNode continues a navigation from a node in the Dawg,
// enumerating through outgoing edges until the navigator is
// satisfied
func (nav *Navigation) FromNode(n *Node, matched string) {
	iter := nav.dawg.iterNodeCache.iterNode(n)
	for i := 0; i < len(iter); i++ {
		state := &iter[i]
		if nav.navigator.PushEdge(state.prefix) {
			// The navigator wants us to enter this edge
			nav.FromEdge(state, matched)
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
func (nav *Navigation) FromEdge(state *navState, matched string) {
	if state.prefix == NoPrefix {
		return
	}
	navigator := nav.navigator
	if !navigator.Accepts(state.prefix) {
		// The navigator doesn't want this prefix letter:
		// we're done
		return
	}
	// The navigator wants this prefix letter:
	// add it to the matched prefix and find out whether
	// it is now in a final state (i.e. an entire valid word)
	matched += string(state.prefix)
	final := false
	if state.nextNode == nil || state.nextNode.IsWord {
		final = true
	}
	// Notify the navigator of the match
	if nav.isResumable {
		// We want the full navigation state to be passed to navigator.Accept()
		navigator.Accept(
			matched,
			final,
			// Create a navState that would resume the navigation at our
			// current location within the prefix, with the same nextNode
			&navState{prefix: state.prefix, nextNode: state.nextNode},
		)
	} else {
		// No need to pass the full state
		navigator.Accept(matched, final, nil)
	}
	if state.nextNode != nil && navigator.IsAccepting() {
		// Completed a whole prefix and still the navigator
		// has appetite: continue to the following node
		nav.FromNode(state.nextNode, matched)
	}
}

// Resume continues a navigation on the underlying Dawg
// using the given Navigator, from a previously saved navigation
// state
func (nav *Navigation) Resume(dawg *DAWG, navigator Navigator, state *navState, matched string) {
	nav.dawg = dawg
	nav.navigator = navigator
	if navigator.IsAccepting() {
		nav.FromEdge(state, matched)
	}
	navigator.Done()
}

// FindLeftParts returns all left part permutations that can be generated
// from the given rack, grouped by length
func FindLeftParts(dawg *DAWG, rack string) [][]*LeftPart {
	var lpn LeftPermutationNavigator
	lpn.Init(rack)
	dawg.NavigateResumable(&lpn)
	return lpn.leftParts
}
