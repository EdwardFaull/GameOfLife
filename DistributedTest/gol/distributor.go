package gol

import (
	"fmt"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	keyPresses <-chan rune

	workerEvents         chan Event
	workerKeyPresses     []chan rune
	fillers              []chan filler
	globalFiller         chan filler
	turnFinishedChannels []chan bool
	ticker               <-chan bool
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, world [][]byte) ([]util.Cell, int) {

	//TODO: Initialise semaphores for locking finished workers
	turn := 0
	threadHeight := float32(p.ImageHeight) / float32(p.Threads)

	//TODO: Split image, send worker goroutines
	for t := 0; t < p.Threads; t++ {
		endY := int(float32(t+1) * threadHeight)
		if t == p.Threads-1 {
			endY = p.ImageHeight
		}
		startY := int(float32(t) * threadHeight)

		fillerElement := make(chan filler, p.Threads)
		c.fillers[t] = fillerElement

		finishedChannel := make(chan bool)
		c.turnFinishedChannels[t] = finishedChannel

		keyPress := make(chan rune, 10)
		c.workerKeyPresses[t] = keyPress

		workerParams := workerParams{
			StartY:      startY,
			EndY:        endY,
			ImageWidth:  p.ImageWidth,
			ImageHeight: endY - startY,
			Turns:       p.Turns,
		}
		workerChannels := workerChannels{
			events:          c.workerEvents,
			globalFiller:    c.globalFiller,
			workerFiller:    fillerElement,
			finishedChannel: finishedChannel,
			keyPresses:      keyPress,
		}
		go worker(world[startY:endY], workerParams, workerChannels, t)
	}

	aliveCells, turn := handleChannels(p, c)
	fmt.Println(aliveCells, turn)
	// Make sure that the Io has finished any output before exiting.
	//c.ioCommand <- ioCheckIdle

	//c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	//close(c.events)

	return aliveCells, turn
}

func sendLinesToWorker(p Params, f filler, c distributorChannels) {
	worker := f.workerID
	upperWorker := (worker + 1) % p.Threads
	lowerWorker := (worker - 1)
	if lowerWorker < 0 {
		lowerWorker = lowerWorker + p.Threads
	}
	c.fillers[upperWorker] <- filler{lowerLine: nil, upperLine: f.upperLine, workerID: worker}
	c.fillers[lowerWorker] <- filler{lowerLine: f.lowerLine, upperLine: nil, workerID: worker}
}

func handleChannels(p Params, c distributorChannels) ([]util.Cell, int) {
	isDone := false
	aliveCells := []util.Cell{}
	savingAliveCells := []util.Cell{}

	workersCompletedTurn := 0
	workersFinished := 0
	turn := 0
	isPaused := false
	isSaving := false
	imageStripsSaved := 0

	//prevTurnAliveCellCount := 0
	workingAliveCellCount := 0

	for {
		select {
		case event := <-c.workerEvents:
			switch e := event.(type) {
			case WorkerTurnComplete:
				workersCompletedTurn++
				workingAliveCellCount += e.CellsCount
				if workersCompletedTurn == p.Threads {
					workersCompletedTurn = 0
					//prevTurnAliveCellCount = workingAliveCellCount
					workingAliveCellCount = 0
					c.events <- TurnComplete{CompletedTurns: turn}
					(turn)++
					//Send all clear to workers to start next turn
					for i := 0; i < p.Threads; i++ {
						c.turnFinishedChannels[i] <- true
					}
				}
			case WorkerFinalTurnComplete:
				workersFinished++
				aliveCells = append(aliveCells, e.Alive...)
				if workersFinished == p.Threads {
					c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: aliveCells}
					isDone = true
					fmt.Println("Done.")
				}
			case CellFlipped:
				//c.events <- event
			case WorkerSaveImage:
				savingAliveCells = append(savingAliveCells, e.Alive...)
				imageStripsSaved++
				if imageStripsSaved == p.Threads {
					//fmt.Println("Received alive Cells,", savingAliveCells)
					//outputImage(p, c, savingAliveCells, turn)
					imageStripsSaved = 0
					isSaving = false
				}
			}
		case f := <-c.globalFiller:
			sendLinesToWorker(p, f, c)
		case k := <-c.keyPresses:
			switch k {
			case 'p':
				for _, kp := range c.workerKeyPresses {
					kp <- k
				}
				isPaused = !isPaused
				if isPaused {
					c.events <- StateChange{turn, Paused}
				} else {
					c.events <- StateChange{turn, Executing}
				}
			case 's':
				if !isSaving {
					for _, kp := range c.workerKeyPresses {
						kp <- k
					}
				}
				isSaving = !isSaving
			case 'q':
				if !isSaving {
					for _, kp := range c.workerKeyPresses {
						kp <- k
					}
				}
				isSaving = !isSaving
				c.events <- StateChange{turn, Quitting}
				return aliveCells, turn
			}
		case <-c.ticker:
			fmt.Println("Distributor received ticker")
			c.events <- AliveCellsCount{CompletedTurns: turn, CellsCount: len(aliveCells)}
			fmt.Println("Distributor sent event")
		}
		if isDone {
			//ticker.Stop()
			return aliveCells, turn
		}
	}
}

/*
func saveImage(p Params, c distributorChannels, turns int) {
	aliveCells := []util.Cell{}
	for i := 0; i < p.Threads; i++ {
		select {
		case event := <-c.workerEvents:
			switch e := event.(type) {
			case WorkerSaveImage:
				aliveCells = append(aliveCells, e.Alive...)
				fmt.Println("Received alive Cells,", i, e.Alive)
			}
		}
	}
	outputImage(p, c, aliveCells, turns)
}

func outputImage(p Params, c distributorChannels, aliveCells []util.Cell, turns int) {
	//c.ioCommand <- ioOutput
	s := fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, turns)
	c.filename <- s

	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
		for j := range world[i] {
			world[i][j] = 0
		}
	}

	for _, cell := range aliveCells {
		world[cell.Y][cell.X] = 255
	}

	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			c.output <- world[i][j]
		}
	}
}
*/
