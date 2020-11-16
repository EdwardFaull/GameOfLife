package gol

import "uk.ac.bris.cs/gameoflife/util"

type workerParams struct {
	StartX      int
	StartY      int
	EndX        int
	EndY        int
	ImageWidth  int
	ImageHeight int
	Turns       int
}

type workerChannels struct {
	events chan<- Event
}

func worker(world [][]byte, p workerParams, c workerChannels) {

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
		//TODO: Semaphores
		world = calculateNextState(p, world, c, x)
		c.events <- WorkerTurnComplete{CompletedTurns: x}
		turn = x
	}
	aliveCells := calculateAliveCells(p, world)
	c.events <- WorkerFinalTurnComplete{CompletedTurns: turn, Alive: aliveCells}

}

func calculateNextState(p workerParams, world [][]byte, c workerChannels, completedTurns int) [][]byte {
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

func sendFlippedEvent(x int, y int, completedTurns int, c workerChannels) {
	cell := util.Cell{X: x, Y: y}
	c.events <- CellFlipped{CompletedTurns: completedTurns, Cell: cell}
}

func calculateAliveCells(p workerParams, world [][]byte) []util.Cell {
	alive := []util.Cell{}
	for y := p.StartY; y < p.EndY; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
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
