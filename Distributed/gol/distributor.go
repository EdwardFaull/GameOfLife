package gol

import (
	"fmt"

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
	killChan             <-chan bool
	killConfirmChan      chan<- bool
	workerKillChan       []chan bool
}

// distributor divides the work between workers and interacts with other goroutines.
func Distributor(p Params, alive []util.Cell, events chan Event, keyPressEvents chan Event, keyPresses chan rune,
	ticker chan bool, killChan <-chan bool, killConfirmChan chan<- bool) ([]util.Cell, int) {
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}
	for _, c := range alive {
		world[c.Y][c.X] = 255
	}

	workerEvents := make(chan Event, 1000)
	globalFiller := make(chan filler, 10)

	c := distributorChannels{
		events:               events,
		keyPressEvents:       keyPressEvents,
		keyPresses:           keyPresses,
		workerEvents:         workerEvents,
		workerKeyPresses:     make([]chan rune, p.Threads),
		fillers:              make([]chan filler, p.Threads),
		globalFiller:         globalFiller,
		turnFinishedChannels: make([]chan int, p.Threads),
		ticker:               ticker,
		killChan:             killChan,
		killConfirmChan:      killConfirmChan,
		workerKillChan:       make([]chan bool, p.Threads),
	}

	fmt.Println("Began new GoL")
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

		fillerElement := make(chan filler, 10)
		c.fillers[t] = fillerElement

		finishedChannel := make(chan int)
		c.turnFinishedChannels[t] = finishedChannel

		keyPress := make(chan rune, 10)
		c.workerKeyPresses[t] = keyPress

		killChan := make(chan bool)
		c.workerKillChan[t] = killChan

		workerParams := workerParams{
			StartY:      startY,
			EndY:        endY,
			ImageWidth:  p.ImageWidth,
			ImageHeight: endY - startY,
			Turns:       p.Turns,
		}
		workerChannels := workerChannels{
			events:          workerEvents,
			globalFiller:    globalFiller,
			workerFiller:    fillerElement,
			finishedChannel: finishedChannel,
			keyPresses:      keyPress,
			killChan:        killChan,
		}
		go worker(world[startY:endY], workerParams, workerChannels, t)
	}

	aliveCells, turn := handleChannels(p, c)
	// Make sure that the Io has finished any output before exiting.
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
	workersCompletedTurn := 0
	workersFinished := 0
	turn := 0
	isPaused := false

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
					turn++
					//Send all clear to workers to start next turn
					for i := 0; i < p.Threads; i++ {
						c.turnFinishedChannels[i] <- turn
					}
					//fmt.Println("TURN COMPLETE")
				}
			case WorkerFinalTurnComplete:
				workersFinished++
				aliveCells = append(aliveCells, e.Alive...)
				if workersFinished == p.Threads {
					c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: aliveCells}
					isDone = true
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
					c.keyPressEvents <- StateChange{turn, Paused, prevTurnAliveCells}
				} else {
					c.keyPressEvents <- StateChange{turn, Executing, nil}
				}
				fmt.Println("Game paused.")
			case 's':
				c.keyPressEvents <- StateChange{turn, Saving, prevTurnAliveCells}
			case 'q':
				c.keyPressEvents <- StateChange{turn, Quitting, prevTurnAliveCells}
				for _, kp := range c.workerKeyPresses {
					kp <- k
				}
				isPaused = true
			case 'r':
				for _, kp := range c.workerKeyPresses {
					kp <- k
				}
				isPaused = false
			}
		case <-c.ticker:
			fmt.Println("Sent update to channel")
			c.events <- AliveCellsCount{CompletedTurns: turn, CellsCount: len(prevTurnAliveCells), Alive: prevTurnAliveCells}
		case <-c.killChan:
			for _, e := range c.workerKillChan {
				e <- true
			}
			c.killConfirmChan <- true
			return aliveCells, turn
		}
		if isDone {
			//ticker.Stop()
			return aliveCells, turn
		}
	}
}

/*

 */
