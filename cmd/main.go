package main

import (
	"flag"
	"fmt"
	"time"

	"scrabble/pkg/scrabble"
)

var numGames = flag.Int("n", 10, "Number of games to simulate")

func main() {
	start := time.Now()
	flag.Parse()

	dict := scrabble.NewDictionary()
	tileSet := scrabble.DefaultTileSet

	dawg := scrabble.NewDawg(dict)

	var winsA, winsB int

	for i := 0; i < *numGames; i++ {
		scoreA, scoreB := simulateGame(tileSet, dawg)
		if scoreA > scoreB {
			winsA++
		}
		if scoreB > scoreA {
			winsB++
		}
	}

	elapsed := time.Since(start)
	fmt.Printf("%v games were played\nRobot A won %v games, and Robot B won %v games; %v games were draws.\n",
		*numGames,
		winsA,
		winsB,
		*numGames-winsA-winsB,
	)
	fmt.Println("Took", elapsed)
}

func simulateGame(tileSet *scrabble.TileSet, dawg *scrabble.DAWG) (scoreA, scoreB int) {
	g := scrabble.NewGame(tileSet, dawg)

	bot1 := scrabble.NewBot(scrabble.NewPlayer("Alphonse", g.Bag), &scrabble.HighScore{})
	bot2 := scrabble.NewBot(scrabble.NewPlayer("Sylvestre", g.Bag), &scrabble.HighScore{})
	g.Players[0], g.Players[1] = bot1.Player, bot2.Player

	for i := 0; ; i++ {
		state := g.State()
		var move scrabble.Move
		// Ask robotA or robotB to generate a move
		if i%2 == 0 {
			move = bot1.GenerateMove(state)
		} else {
			move = bot2.GenerateMove(state)
		}
		g.ApplyValid(move)
		// fmt.Println(move)
		if g.IsOver() {
			// fmt.Println(g.Board)
			fmt.Printf("Game over!\n\n")
			break
		}
	}
	scoreA, scoreB = g.Players[0].Score, g.Players[1].Score
	return scoreA, scoreB
}
