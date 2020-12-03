package gol

import (
	"uk.ac.bris.cs/gameoflife/util"
)

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune, keyPressEvents chan<- Event,
	alive []util.Cell, ticker <-chan bool) ([]util.Cell, int) {

	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}
	for _, c := range alive {
		world[c.Y][c.X] = 255
	}

	distributorChannels := distributorChannels{
		events,
		keyPressEvents,
		keyPresses,
		make(chan Event),
		make([]chan rune, p.Threads),
		make([]chan filler, p.Threads),
		make(chan filler),
		make([]chan int, p.Threads),
		ticker,
	}
	return distributor(p, distributorChannels, world)
}
