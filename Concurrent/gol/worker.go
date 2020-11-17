package gol

import (
	"uk.ac.bris.cs/gameoflife/util"
)

type workerParams struct {
	StartY      int
	EndY        int
	ImageWidth  int
	ImageHeight int
	Turns       int
}

type workerChannels struct {
	events       chan<- Event
	globalFiller chan<- filler
	workerFiller <-chan filler
}

//Used to send the top and bottom arrays of each worker's world to the distributor,
//as well as receive them from it
type filler struct {
	lowerLine []byte
	upperLine []byte
	workerID  int
}

func worker(world [][]byte, p workerParams, c workerChannels, workerID int) {

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

		//Send top and bottom arrays to distributor
		c.globalFiller <- filler{lowerLine: world[0], upperLine: world[p.ImageHeight-1], workerID: workerID}

		//Receive lines outside world's boundaries for use in this worker
		receivedFiller := <-c.workerFiller
		receivedFiller2 := <-c.workerFiller
		//fmt.Println(workerID, "Recieved fillers from workers", receivedFiller.workerID, receivedFiller2.workerID)
		//Decide which filler delivered which line
		upperLine := []byte{}
		lowerLine := []byte{}
		if receivedFiller.upperLine != nil {
			upperLine = receivedFiller.upperLine
			lowerLine = receivedFiller2.lowerLine
		} else {
			upperLine = receivedFiller2.upperLine
			lowerLine = receivedFiller.lowerLine
		}
		//Execute turn of game
		world = calculateNextState(p, world, c, x, upperLine, lowerLine)
		//Send completion event to distributor
		c.events <- WorkerTurnComplete{CompletedTurns: x}
		turn = x
	}
	aliveCells := calculateAliveCells(p, world)
	c.events <- WorkerFinalTurnComplete{CompletedTurns: turn, Alive: aliveCells}
}

func calculateNextState(p workerParams, world [][]byte, c workerChannels, completedTurns int, upperLine []byte, lowerLine []byte) [][]byte {
	h := p.ImageHeight
	w := p.ImageWidth
	nworld := make([][]byte, h, w)
	for y, a := range world {
		nA := make([]byte, w)
		for x, b := range a {
			nB := byte(0)
			ln := calculateAliveNeighbours(h, w, world, x, y, upperLine, lowerLine)
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
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				coord := util.Cell{X: x, Y: y}
				alive = append(alive, coord)
			}
		}
	}
	return alive
}

func calculateAliveNeighbours(h int, w int, world [][]byte, x int, y int, upperLine []byte, lowerLine []byte) int {
	ans := 0
	up := y + 1
	down := y - 1

	right := (x + 1) % w
	left := (x - 1) % w
	if left < 0 {
		left = left + w
	}

	neighbours := make([][]byte, 3)
	neighbours[1] = []byte{world[y][left], world[y][x], world[y][right]}
	if y == 0 {
		neighbours[2] = []byte{upperLine[left], upperLine[x], upperLine[right]}
	} else {
		neighbours[2] = []byte{world[down][left], world[down][x], world[down][right]}
	}
	if y == h-1 {
		neighbours[0] = []byte{lowerLine[left], lowerLine[x], lowerLine[right]}
	} else {
		neighbours[0] = []byte{world[up][left], world[up][x], world[up][right]}
	}

	up = 0
	down = 2
	left = 0
	right = 2
	middle := 1

	if neighbours[up][middle] == 255 {
		ans++
	}
	if neighbours[down][middle] == 255 {
		ans++
	}
	if neighbours[middle][left] == 255 {
		ans++
	}
	if neighbours[middle][right] == 255 {
		ans++
	}
	if neighbours[down][right] == 255 {
		ans++
	}
	if neighbours[up][right] == 255 {
		ans++
	}
	if neighbours[up][left] == 255 {
		ans++
	}
	if neighbours[down][left] == 255 {
		ans++
	}
	return ans
}
