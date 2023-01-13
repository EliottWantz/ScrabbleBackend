package scrabble

const MaxPassMoves int = 6

type Game struct {
	Players      [2]*Player
	Board        *Board
	Bag          *Bag
	DAWG         *DAWG
	TileSet      *TileSet
	MoveList     []*MoveItem
	Finished     bool
	NumPassMoves int
}

// GameState contains the bare minimum of information
// that is needed for a robot player to decide on a move
// in a Game.
type GameState struct {
	DAWG            *DAWG
	TileSet         *TileSet
	Board           *Board
	Rack            *Rack
	ExchangeAllowed bool
}

// MoveItem is an entry in the MoveList of a Game.
// It contains the player's Rack as it was before the move,
// as well as the move itself.
type MoveItem struct {
	RackBefore string
	Move       Move
}

func NewGame(tileSet *TileSet, dawg *DAWG) *Game {
	g := &Game{
		Board:   NewBoard(),
		DAWG:    dawg,
		Bag:     NewBag(tileSet),
		TileSet: tileSet,
	}

	return g
}

func (g *Game) AddPlayer(p *Player) {
	g.Players[0] = p
}

// PlayerToMoveIndex returns 0 or 1 depending on which player's move it is
func (g *Game) PlayerToMoveIndex() int {
	return len(g.MoveList) % 2
}

// PlayerToMove returns the player which player's move it is
func (g *Game) PlayerToMove() *Player {
	return g.Players[g.PlayerToMoveIndex()]
}

// PlayTile moves a tile from the player's rack to the board
func (g *Game) PlayTile(t *Tile, pos Position, r *Rack) error {
	err := r.Remove(t.Letter)
	if err != nil {
		return err
	}

	err = g.Board.PlaceTile(t, pos)
	if err != nil {
		return err
	}

	return nil
}

// ApplyValid applies an already validated Move to a Game,
// appends it to the move list, replenishes the player's Rack
// if needed, and updates scores.
func (g *Game) ApplyValid(move Move) error {
	// Be careful to call PlayerToMove() before appending
	// a move to the move list (this reverses the players)
	playerToMove := g.PlayerToMove()
	rackBefore := playerToMove.Rack.AsString()
	if err := move.Apply(g); err != nil {
		// Should not happen because it should be a valid move
		return err
	}

	// Update the scores and append to the move list
	g.scoreMove(rackBefore, move)
	if g.IsOver() {
		// The game is now over: add the FinalMoves
		rackPlayer := playerToMove.Rack.AsString()
		rackOpp := g.Players[1-g.PlayerToMoveIndex()].Rack.AsString()

		multiplyFactor := 2
		if len(rackPlayer) > 0 {
			// The game is not finishing by the final player
			// completing his rack: both players then get the
			// opponent's remaining tile scores
			multiplyFactor = 1
		}
		// Add a final move for the finishing player
		g.scoreMove(rackPlayer, NewFinalMove(rackOpp, multiplyFactor))
		// Add a final move for the opponent
		g.scoreMove(rackOpp, NewFinalMove(rackPlayer, multiplyFactor))
	}
	return nil
}

// scoreMove updates the scores and appends a given Move
// to the Game's MoveList
func (g *Game) scoreMove(rackBefore string, move Move) {
	// Calculate the score
	score := move.Score(g.State())
	// Update the player's score
	g.PlayerToMove().Score += score
	// Append to the move list
	moveItem := &MoveItem{RackBefore: rackBefore, Move: move}
	g.MoveList = append(g.MoveList, moveItem)
}

// IsOver returns true if the Game is over after the last
// move played
func (g *Game) IsOver() bool {
	i := len(g.MoveList)
	if i == 0 {
		// No moves yet: cannot be over
		return false
	}
	// TODO: Check for resignation
	if g.NumPassMoves == MaxPassMoves {
		return true
	}
	lastPlayer := 1 - (i % 2)

	return g.Players[lastPlayer].Rack.IsEmpty()
}

// State returns a new GameState instance describing the state of the
// game in a minimal manner so that a robot player can decide on a move
func (g *Game) State() *GameState {
	return &GameState{
		DAWG:            g.DAWG,
		TileSet:         g.TileSet,
		Board:           g.Board,
		Rack:            g.PlayerToMove().Rack,
		ExchangeAllowed: g.Bag.ExchangeAllowed(),
	}
}

func (gs *GameState) GenerateMoves() []Move {
	leftParts := gs.DAWG.FindLeftParts(gs.Rack.AsString())

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

func (gs *GameState) GenerateMovesOnAxis(index int, horizontal bool, leftParts [][]*LeftPart, resultsChan chan<- []Move) {
	var axis Axis
	axis.Init(gs, index, horizontal)
	// Generate a list of moves and send it on the result channel
	resultsChan <- axis.GenerateMoves(leftParts)
}
