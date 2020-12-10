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
	Threads     int
}

type workerChannels struct {
	events            chan<- Event
	distributorEvents <-chan Event
	globalFiller      chan<- Filler
	workerFiller      <-chan Filler
	finishedChannel   <-chan int
	keyPresses        <-chan rune
	killChan          chan bool
	fetchRequest      chan<- bool
	fetchResponse     <-chan []byte
}

//Used to send the top and bottom arrays of each worker's world to the distributor,
//as well as receive them from it
type Filler struct {
	lowerLine []byte
	upperLine []byte
	workerID  int
}

func (f *Filler) GetLowerLine() []byte {
	return f.lowerLine
}
func (f *Filler) GetUpperLine() []byte {
	return f.upperLine
}

func worker(world [][]byte, p workerParams, c workerChannels, workerID int) ([][]byte, int) {

	isPaused := false
	//For all initially alive cells send a CellFlipped Event.
	/*for y, elem := range world {
		for x, cell := range elem {
			if cell == 255 {
				d := util.Cell{X: x, Y: y + p.StartY}
				cellFlip := CellFlipped{CompletedTurns: 0, Cell: d}
				c.events <- cellFlip
			}
		}
	}
	*/
	turn := 0
	aliveCells := calculateAliveCells(p, world, workerID)

	//Executes all turns of the Game of Life.
	for {
		//Send top and bottom arrays to distributor
		if !isPaused {
			if turn > p.Turns || (turn == 0 && p.Turns == 0) {
				break
			}
			aliveCells = []util.Cell{}
			upperLine, lowerLine := getFillers(p, c, world, workerID)
			//Execute turn of game
			world, aliveCells = calculateNextState(workerID, p, world, c, turn, upperLine, lowerLine)
			//Send completion event to distributor
			c.events <- WorkerTurnComplete{CompletedTurns: turn, Alive: aliveCells}
			turn++
		}
		select {
		case k := <-c.keyPresses:
			switch k {
			case 'p':
				isPaused = !isPaused
			case 's':
				c.events <- WorkerSaveImage{CompletedTurns: turn, Alive: aliveCells}
			case 'q':
				c.events <- WorkerSaveImage{CompletedTurns: turn, Alive: aliveCells}
				isPaused = true
			case 'r':
				isPaused = false
			}
		case <-c.killChan:
			return world, turn
		case x := <-c.finishedChannel:
			turn = x
		}
	}
	c.events <- WorkerFinalTurnComplete{CompletedTurns: turn, Alive: aliveCells}
	return world, turn
}

func getFillers(p workerParams, c workerChannels, world [][]byte, workerID int) ([]byte, []byte) {
	c.globalFiller <- Filler{lowerLine: world[0], upperLine: world[p.ImageHeight-1], workerID: workerID}

	upperLine := []byte{}
	lowerLine := []byte{}
	if workerID == 0 || workerID == p.Threads-1 {
		c.fetchRequest <- true
		filler := <-c.fetchResponse
		if workerID == 0 {
			upperLine = filler
			receivedFiller := <-c.workerFiller
			if p.Threads != 1 {
				lowerLine = receivedFiller.lowerLine
			}
		}
		if workerID == p.Threads-1 {
			lowerLine = filler
			receivedFiller := <-c.workerFiller
			if p.Threads != 1 {
				upperLine = receivedFiller.upperLine
			}
		}
	} else {
		//Receive lines outside world's boundaries for use in this worker
		receivedFiller := <-c.workerFiller
		receivedFiller2 := <-c.workerFiller
		upperID := (workerID + 1) % p.Threads
		//Decide which filler delivered which line
		if receivedFiller.workerID != upperID {
			upperLine = receivedFiller.upperLine
			lowerLine = receivedFiller2.lowerLine
		} else {
			upperLine = receivedFiller2.upperLine
			lowerLine = receivedFiller.lowerLine
		}
	}
	return upperLine, lowerLine
}

func createNewWorld(world [][]byte, p workerParams) [][]byte {
	newWorld := make([][]byte, p.ImageHeight)
	for i := range newWorld {
		newWorld[i] = make([]byte, p.ImageWidth)
		for j := range newWorld[i] {
			newWorld[i][j] = world[i][j]
		}
	}
	return newWorld
}

func duplicateArray(array []byte, p workerParams) []byte {
	newArray := make([]byte, p.ImageWidth)
	for i := range newArray {
		newArray[i] = array[i]
	}
	return newArray
}

func calculateNextState(id int, p workerParams, world [][]byte, c workerChannels,
	completedTurns int, upperLine []byte, lowerLine []byte) ([][]byte, []util.Cell) {
	aliveCells := []util.Cell{}
	h := p.ImageHeight
	w := p.ImageWidth
	nworld := make([][]byte, h, w)
	for y, a := range world {
		nA := make([]byte, w)
		for x, b := range a {
			nB := byte(0)
			ln := calculateAliveNeighbours(id, h, w, world, x, y, upperLine, lowerLine)
			if b == 0 {
				if ln == 3 {
					nB = 255
					aliveCells = append(aliveCells, util.Cell{X: x, Y: y + p.StartY})
					sendFlippedEvent(x, y+p.StartY, completedTurns, c)
				} else {
					nB = 0
				}
			} else {
				if ln < 2 || ln > 3 {
					nB = 0
					sendFlippedEvent(x, y+p.StartY, completedTurns, c)
				} else {
					aliveCells = append(aliveCells, util.Cell{X: x, Y: y + p.StartY})
					nB = 255
				}
			}
			nA[x] = nB
		}
		nworld[y] = nA
	}
	return nworld, aliveCells
}

func sendFlippedEvent(x int, y int, completedTurns int, c workerChannels) {
	cell := util.Cell{X: x, Y: y}
	c.events <- CellFlipped{CompletedTurns: completedTurns, Cell: cell}
}

func calculateAliveCells(p workerParams, world [][]byte, workerID int) []util.Cell {
	alive := []util.Cell{}
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				coord := util.Cell{X: x, Y: y + p.StartY}
				alive = append(alive, coord)
			}
		}
	}
	return alive
}

func calculateAliveNeighbours(id int, h int, w int, world [][]byte, x int, y int, upperLine []byte, lowerLine []byte) int {
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
