package scrabble

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
	Dawg            *DAWG
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
	game := &Game{
		Board:   NewBoard(),
		DAWG:    dawg,
		Bag:     NewBag(tileSet),
		TileSet: tileSet,
	}

	return game
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
func (g *Game) PlayTile(tile *Tile, pos Position) bool {
	sq := g.Board.GetSquare(pos)
	if sq == nil {
		// No such square
		return false
	}
	if sq.Tile != nil {
		// We already have a tile in this location
		return false
	}
	playerToMove := g.PlayerToMove()
	if !playerToMove.Rack.Remove(tile.Letter) {
		// This tile isn't in the rack
		return false
	}
	sq.Tile = tile
	return true
}

func (g *Game) PlaceTile(pos Position, tile *Tile) bool {
	sq := g.Board.GetSquare(pos)
	if sq == nil {
		// No such square
		return false
	}
	if sq.Tile != nil {
		// We already have a tile in this location
		return false
	}
	sq.Tile = tile
	return true
}

// ApplyValid applies an already validated Move to a Game,
// appends it to the move list, replenishes the player's Rack
// if needed, and updates scores.
func (game *Game) ApplyValid(move Move) bool {
	// Be careful to call PlayerToMove() before appending
	// a move to the move list (this reverses the players)
	playerToMove := game.PlayerToMove()
	rack := playerToMove.Rack
	rackBefore := rack.AsString()
	if !move.Apply(game) {
		// Not valid! Should not happen...
		return false
	}
	// Update the scores and append to the move list
	game.acceptMove(rackBefore, move)
	// Replenish the player's rack, as needed
	rack.Fill(game.Bag)
	if game.IsOver() {
		// The game is now over: add the FinalMoves
		rackThis := playerToMove.Rack.AsString()
		rackOpp := game.Players[1-game.PlayerToMoveIndex()].Rack.AsString()
		multiplyFactor := 2
		if len(rackThis) > 0 {
			// The game is not finishing by the final player
			// completing his rack: both players then get the
			// opponent's remaining tile scores
			multiplyFactor = 1
		}
		// Add a final move for the opponent
		// (which in most cases yields zero points, since
		// the finishing player has no tiles left)
		finalOpp := NewFinalMove(rackThis, multiplyFactor)
		game.acceptMove(rackOpp, finalOpp)
		// Add a final move for the finishing player
		// (which in most cases yields double the tile scores
		// of the opponent's rack)
		finalThis := NewFinalMove(rackOpp, multiplyFactor)
		game.acceptMove(rackThis, finalThis)
	}
	return true
}

// acceptMove updates the scores and appends a given Move
// to the Game's MoveList
func (game *Game) acceptMove(rackBefore string, move Move) {
	// Calculate the score
	score := move.Score(game.State())
	// Update the player's score
	game.PlayerToMove().Score += score
	// Append to the move list
	moveItem := &MoveItem{RackBefore: rackBefore, Move: move}
	game.MoveList = append(game.MoveList, moveItem)
}

// State returns a new GameState instance describing the state of the
// game in a minimal manner so that a robot player can decide on a move
func (game *Game) State() *GameState {
	return &GameState{
		Dawg:            game.DAWG,
		TileSet:         game.TileSet,
		Board:           game.Board,
		Rack:            game.PlayerToMove().Rack,
		ExchangeAllowed: game.Bag.ExchangeAllowed(),
	}
}

// IsOver returns true if the Game is over after the last
// move played
func (game *Game) IsOver() bool {
	ix := len(game.MoveList)
	if ix == 0 {
		// No moves yet: cannot be over
		return false
	}
	// TODO: Check for resignation
	if game.NumPassMoves == 6 {
		// Six consecutive zero-point moves
		// (e.g. three rounds of passes) finish the game
		return true
	}
	lastPlayer := 1 - (ix % 2)

	return game.Players[lastPlayer].Rack.IsEmpty()
}
