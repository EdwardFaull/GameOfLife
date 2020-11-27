package gol

import "uk.ac.bris.cs/gameoflife/util"

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune, alive []util.Cell) ([]util.Cell, int) {

	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}
	for i := range world {
		for j := range world[i] {
			for _, c := range alive {
				if c.X == j && c.Y == i {
					world[i][j] = 255
				} else {
					world[i][j] = 0
				}
			}
		}
	}

	distributorChannels := distributorChannels{
		events,
		keyPresses,
		make(chan Event),
		make([]chan rune, p.Threads),
		make([]chan filler, p.Threads),
		make(chan filler),
		make([]chan bool, p.Threads),
	}
	return distributor(p, distributorChannels, world)
}
