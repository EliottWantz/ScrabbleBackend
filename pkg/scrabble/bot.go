package scrabble

import (
	"math/rand"
	"sort"
)

// Robot is an interface for automatic players that implement
// a playing strategy to pick a move given a list of legal tile
// moves.
type Strategy interface {
	PickMove(state *GameState, moves []Move) Move
}

type Bot struct {
	*Player
	Strategy
}

// GenerateMove generates a list of legal tile moves, then picks a move from
// with the current bot's strategy
func (b *Bot) GenerateMove(state *GameState) Move {
	moves := state.GenerateMoves()
	return b.PickMove(state, moves)
}

// HighScore strategy always picks the highest-scoring move available, or
// exchanges all tiles if there is no valid tile move, or passes if exchange is
// not allowed.
type HighScore struct{}

// OneOfNBest picks one of the N highest-scoring moves at random.
type OneOfNBest struct {
	N int
}

// Sort the moves by score
type byScore struct {
	state *GameState
	moves []Move
}

func (list byScore) Len() int {
	return len(list.moves)
}

func (list byScore) Swap(i, j int) {
	list.moves[i], list.moves[j] = list.moves[j], list.moves[i]
}

func (list byScore) Less(i, j int) bool {
	// We want descending order, so we reverse the comparison
	return list.moves[i].Score(list.state) > list.moves[j].Score(list.state)
}

// PickMove for a HighScore picks the highest scoring move available,
// or an exchange move, or a pass move as a last resort
func (hs *HighScore) PickMove(state *GameState, moves []Move) Move {
	if len(moves) > 0 {
		// Sort by score and return the highest scoring move
		sort.Sort(byScore{state, moves})
		return moves[0]
	}
	// No valid tile moves
	if state.ExchangeAllowed {
		// Exchange all tiles, since that is allowed
		return NewExchangeMove(state.Rack.AsString())
	}
	// Exchange forbidden: Return a pass move
	return NewPassMove()
}

// PickMove for OneOfNBestRobot selects one of the N highest-scoring
// moves at random, or an exchange move, or a pass move as a last resort
func (ofb *OneOfNBest) PickMove(state *GameState, moves []Move) Move {
	if len(moves) > 0 {
		// Sort by score
		sort.Sort(byScore{state, moves})
		// Cut the list down to N, if it is longer than that
		if len(moves) > ofb.N {
			moves = moves[:ofb.N]
		}
		// # nosec
		pick := rand.Intn(len(moves))
		// Pick a move by random from the remaining list
		return moves[pick]
	}
	// No valid tile moves
	if state.ExchangeAllowed {
		// Exchange all tiles, since that is allowed
		return NewExchangeMove(state.Rack.AsString())
	}
	// Exchange forbidden: Return a pass move
	return NewPassMove()
}

func NewBot(p *Player, s Strategy) *Bot {
	return &Bot{
		Player:   p,
		Strategy: s,
	}
}
