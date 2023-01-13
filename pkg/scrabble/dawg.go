package scrabble

import (
	"errors"
	"sync"
)

var (
	ErrNodeNotFound = errors.New("dawg node not found")
	ErrNodeIsNil    = errors.New("node is nil")
)

type DAWG struct {
	Root          *Node
	iterNodeCache iterNodeCache
	crossCache    crossCache
}

type Node struct {
	IsWord bool
	Edges  map[rune]*Node
}

// iterNodeCache is a cached map of nodes to navState
// It helps to not traverse the dawg again when already traversed
type iterNodeCache struct {
	mu    sync.Mutex
	cache map[*Node][]navState
}

// crossCache stores the available letters for a given key
// A key is like all* where the available words are "alla", "alle" and "allo",
// so the runes are ['a', 'e', 'o']
type crossCache struct {
	mu    sync.Mutex
	cache map[string][]rune
}

func NewNode() *Node {
	return &Node{
		Edges: make(map[rune]*Node),
	}
}

func NewDawg(dict *Dictionary) *DAWG {
	d := &DAWG{
		Root: NewNode(),
		iterNodeCache: iterNodeCache{
			cache: make(map[*Node][]navState),
		},
		crossCache: crossCache{
			cache: make(map[string][]rune),
		},
	}

	for _, word := range dict.Words {
		d.insert(word)
	}

	return d
}

func (d *DAWG) insert(word string) {
	curr := d.Root
	for _, letter := range word {
		next, ok := curr.Edges[letter]
		if !ok {
			next = NewNode()
			curr.Edges[letter] = next
		}
		curr = next
	}

	curr.IsWord = true
}

// IsWord attempts to find a word in a DAWG, returning true if
// found or false if not.
func (d *DAWG) IsWord(word string) bool {
	var fn FindNavigator
	fn.Init(word)
	d.Navigate(&fn)
	return fn.found
}

// Match returns all words in the Dawg that match a
// given pattern string, which can include '*' wildcards/blanks.
func (d *DAWG) Match(pattern string) []string {
	var mn MatchNavigator
	mn.Init(pattern)
	d.Navigate(&mn)
	return mn.results
}

// NavigateResumable performs a resumable navigation through the DAWG under the
// control of a Navigator
func (d *DAWG) NavigateResumable(navigator Navigator) {
	var nav Navigation
	nav.isResumable = true
	nav.Go(d, navigator)
}

// Navigate performs a navigation through the DAWG under the
// control of a Navigator
func (d *DAWG) Navigate(navigator Navigator) {
	var nav Navigation
	nav.Go(d, navigator)
}

const NoPrefix = '_'

// Resume resumes a navigation through the DAWG under the
// control of a Navigator, from a previously saved state
func (d *DAWG) Resume(navigator Navigator, state *navState, matched string) {
	var nav Navigation
	state.prefix = NoPrefix
	nav.Resume(d, navigator, state, matched)
}

// CrossCheck return a slice of allowed letters
// in a cross-check set, given a left/top and right/bottom
// string that intersects the square being checked.
func (d *DAWG) CrossCheck(prev, after string) []rune {
	lenLeft := len(prev)
	key := prev + "*" + after
	fetchFunc := func(key string) []rune {
		// Find all matches for key in DAWG and add the letter corresponding
		// to the wildcard * to the slice of letters.
		matches := d.Match(key)
		// Collect the 'middle' letters (the ones standing in
		// for the wildcard)
		letters := make([]rune, 0, len(matches))
		for _, match := range matches {
			match := []rune(match)
			letters = append(letters, match[lenLeft])
		}
		return letters
	}

	return d.crossCache.lookup(key, fetchFunc)
}

// iterNode traverses through all node edges and adds the navigation state
// to the cache if not already present
func (i *iterNodeCache) iterNode(n *Node) []navState {
	i.mu.Lock()
	defer i.mu.Unlock()
	if result, ok := i.cache[n]; ok {
		return result
	}
	originalNode := n

	result := make([]navState, 0, len(n.Edges))

	for letter, next := range n.Edges {
		result = append(result,
			navState{
				prefix:   letter,
				nextNode: next,
			},
		)
	}

	// Add navigaion result to cache
	i.cache[originalNode] = result

	return result
}

func (cc *crossCache) lookup(key string, fetchFunc func(string) []rune) []rune {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if runes, ok := cc.cache[key]; ok {
		return runes
	}
	runes := fetchFunc(key)
	cc.cache[key] = runes
	return runes
}

// FindLeftParts returns all left part permutations that can be generated
// from the given rack, grouped by length
func (dawg *DAWG) FindLeftParts(rack string) [][]*LeftPart {
	var lpn LeftPermutationNavigator
	lpn.Init(rack)
	dawg.NavigateResumable(&lpn)
	return lpn.leftParts
}
