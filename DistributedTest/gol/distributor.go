package gol

import (
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events         chan<- Event
	keyPressEvents chan<- Event
	keyPresses     <-chan rune

	workerEvents         chan Event
	workerKeyPresses     []chan rune
	fillers              []chan filler
	globalFiller         chan filler
	turnFinishedChannels []chan int
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

		finishedChannel := make(chan int)
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
	// Make sure that the Io has finished any output before exiting.
	//c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)

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

	//finishedTurn := make(chan []util.Cell, 1)

	workersCompletedTurn := 0
	workersFinished := 0
	turn := 0
	isPaused := false
	isSaving := false
	imageStripsSaved := 0

	prevTurnAliveCells := []util.Cell{}
	workingAliveCells := []util.Cell{}

	for {
		select {
		case event := <-c.workerEvents:
			switch e := event.(type) {
			case WorkerTurnComplete:
				workersCompletedTurn++
				workingAliveCells = append(workingAliveCells, e.Alive...)
				if workersCompletedTurn == p.Threads {
					workersCompletedTurn = 0
					prevTurnAliveCells = workingAliveCells
					workingAliveCells = nil
					//c.events <- TurnComplete{CompletedTurns: turn}
					turn++
					//Send all clear to workers to start next turn
					for i := 0; i < p.Threads; i++ {
						c.turnFinishedChannels[i] <- turn
					}
				}
			case WorkerFinalTurnComplete:
				workersFinished++
				aliveCells = append(aliveCells, e.Alive...)
				if workersFinished == p.Threads {
					c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: aliveCells}
					isDone = true
				}
			case WorkerSaveImage:
				savingAliveCells = append(savingAliveCells, e.Alive...)
				imageStripsSaved++
				if imageStripsSaved == p.Threads {
					imageStripsSaved = 0
					isSaving = false
					c.keyPressEvents <- StateChange{turn, Saving, savingAliveCells}
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
					/*for {
						select {
						case t := <-finishedTurn:
							if t == turn {
								cells := <-finishedTurnCells
								c.keyPressEvents <- StateChange{turn, Paused, cells}
							} else {
								<-finishedTurnCells
							}
						}
					}*/
					c.keyPressEvents <- StateChange{turn, Paused, prevTurnAliveCells}
				} else {
					c.keyPressEvents <- StateChange{turn, Executing, nil}
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
				c.keyPressEvents <- StateChange{turn, Quitting, prevTurnAliveCells}
				return aliveCells, turn
			}
		case <-c.ticker:
			c.events <- AliveCellsCount{CompletedTurns: turn, CellsCount: len(prevTurnAliveCells)}
		}
		if isDone {
			//ticker.Stop()
			return aliveCells, turn
		}
	}
}

/*

 */
