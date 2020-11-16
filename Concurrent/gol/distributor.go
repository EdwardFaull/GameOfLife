package gol

import (
	"fmt"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events    chan<- Event
	ioCommand chan<- ioCommand
	ioIdle    <-chan bool
	input     <-chan byte
	output    chan<- byte
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	//Create a 2D slice to store the world.
	world := make([][]byte, p.ImageHeight, p.ImageWidth)

	//TODO: This is implementation uses busy waiting and is bad. Fix.
	c.ioCommand <- ioCheckIdle
	fmt.Println("Sent idle check")
	for {
		idle := false
		select {
		case x := <-c.ioIdle:
			idle = x
		}
		if idle {
			break
		}
	}

	c.ioCommand <- ioInput
	fmt.Println("Sent input signal")
	//TODO: Fix
	for i := 0; i < p.ImageWidth; i++ {
		for j := 0; j < p.ImageHeight; j = j + 0 {
			select {
			case b := <-c.input:
				world[i][j] = b
				j++
			}
		}
	}

	//For all initially alive cells send a CellFlipped Event.
	for i, elem := range world {
		for j, cell := range elem {
			if cell == 255 {
				d := util.Cell{X: i, Y: j}
				cellFlip := CellFlipped{CompletedTurns: 0, Cell: d}
				c.events <- cellFlip
			}
		}
	}

	turn := 0

	//Executes all turns of the Game of Life.
	for x := 0; x < p.Turns; x++ {
		world = calculateNextState(p, world, c, x)
		c.events <- TurnComplete{CompletedTurns: x}
	}
	//TODO: c.events <- FinalTurnComplete{CompletedTurns: x}

	// TODO: Send correct Events when required, e.g. CellFlipped, TurnComplete and FinalTurnComplete.
	//		 See event.go for a list of all events.

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func calculateNextState(p Params, world [][]byte, c distributorChannels, completedTurns int) [][]byte {
	h := p.ImageHeight
	w := p.ImageWidth
	nworld := make([][]byte, h, w)
	for y, a := range world {
		nA := make([]byte, w)
		for x, b := range a {
			nB := byte(0)
			ln := calculateAliveNeighbours(h, w, world, x, y)
			if b == 0 {
				if ln == 3 {
					nB = 255
					sendFlippedEvent(x, y, completedTurns, c)
				} else {
					nB = 0
				}
			} else {
				if ln < 2 || ln > 3 {
					nB = 0
					sendFlippedEvent(x, y, completedTurns, c)
				} else {
					nB = 255
				}
			}
			nA[x] = nB
		}
		nworld[y] = nA
	}
	return nworld
}

func sendFlippedEvent(x int, y int, completedTurns int, c distributorChannels) {
	cell := util.Cell{X: x, Y: y}
	c.events <- CellFlipped{CompletedTurns: completedTurns, Cell: cell}
}

func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	alive := []util.Cell{}
	for y, a := range world {
		for x, b := range a {
			if b == 255 {
				coord := util.Cell{X: x, Y: y}
				alive = append(alive, coord)
			}
		}
	}
	return alive
}

func calculateAliveNeighbours(h int, w int, world [][]byte, x int, y int) int {
	ans := 0
	up := (x + 1) % h
	down := (x - 1) % h
	if down < 0 {
		down = down + h
	}
	right := (y + 1) % w
	left := (y - 1) % w
	if left < 0 {
		left = left + w
	}

	if world[left][x] == 255 {
		ans++
	}
	if world[right][x] == 255 {
		ans++
	}
	if world[y][down] == 255 {
		ans++
	}
	if world[y][up] == 255 {
		ans++
	}
	if world[right][down] == 255 {
		ans++
	}
	if world[right][up] == 255 {
		ans++
	}
	if world[left][up] == 255 {
		ans++
	}
	if world[left][down] == 255 {
		ans++
	}
	return ans
}
