package gol

import (
	"fmt"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events    chan<- Event
	ioCommand chan<- ioCommand
	ioIdle    <-chan bool
	input     <-chan uint8
	output    chan<- uint8
	filename  chan<- string
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	//Create a 2D slice to store the world.
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	//TODO: This is implementation uses busy waiting and is bad.
	c.ioCommand <- ioCheckIdle
	//fmt.Println("Sent idle check")
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

	s := fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)
	c.filename <- s

	//TODO: Fix
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			select {
			case b := <-c.input:
				world[i][j] = b
			}
		}
	}

	//TODO: Initialise semaphores for locking finished workers
	turn := 0
	threadHeight := float32(p.ImageHeight) / float32(p.Threads)
	workerEvents := make(chan Event)
	//Holds the channels that distributor sends boundary arrays needed by each worker
	fillers := make([]chan<- filler, p.Threads, p.Threads)
	//Receives boundary arrays from each worker on each turn for distribution
	globalFiller := make(chan filler, p.Threads)
	//
	turnFinishedChannels := make([]chan bool, p.Threads)

	//TODO: Split image, send worker goroutines
	for t := 0; t < p.Threads; t++ {
		endY := int(float32(t+1) * threadHeight)
		if t == p.Threads-1 {
			endY = p.ImageHeight
		}
		startY := int(float32(t) * threadHeight)

		fillerElement := make(chan filler, p.Threads)
		fillers[t] = fillerElement

		finishedChannel := make(chan bool)
		turnFinishedChannels[t] = finishedChannel

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
		}
		go worker(world[startY:endY], workerParams, workerChannels, t)
	}

	turn = handleChannels(p, c, workerEvents, globalFiller, fillers, turnFinishedChannels)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func sendLinesToWorker(p Params, f filler, channels []chan<- filler) {
	worker := f.workerID
	upperWorker := (worker + 1) % p.Threads
	lowerWorker := (worker - 1)
	if lowerWorker < 0 {
		lowerWorker = lowerWorker + p.Threads
	}
	channels[upperWorker] <- filler{lowerLine: nil, upperLine: f.upperLine, workerID: worker}
	channels[lowerWorker] <- filler{lowerLine: f.lowerLine, upperLine: nil, workerID: worker}
}

func handleChannels(p Params, c distributorChannels, workerEvents <-chan Event,
	globalFiller <-chan filler, fillers []chan<- filler, finishedChannels []chan bool) int {
	isDone := false
	aliveCells := []util.Cell{}

	workersCompletedTurn := 0
	workersFinished := 0
	turn := 0

	for {
		select {
		case event := <-workerEvents:
			switch e := event.(type) {
			case WorkerTurnComplete:
				workersCompletedTurn++
				if workersCompletedTurn == p.Threads {
					workersCompletedTurn = 0
					c.events <- TurnComplete{CompletedTurns: turn}
					(turn)++
					for i := 0; i < p.Threads; i++ {
						finishedChannels[i] <- true
						//semaphore.Post()
					}
					//fmt.Println("======TURN COMPLETE========")
					//TODO: Send all-clear to workers using semaphores
				}
			case WorkerFinalTurnComplete:
				workersFinished++
				aliveCells = append(aliveCells, e.Alive...)
				if workersFinished == p.Threads {
					//TODO: Retrieve alive cells from workers
					c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: aliveCells}
					isDone = true
				}
			}
		case f := <-globalFiller:
			sendLinesToWorker(p, f, fillers)
		}
		if isDone {
			return turn
		}
	}
}
