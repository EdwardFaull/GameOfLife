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
	workersCompletedTurn := 0
	workersFinished := 0
	threadHeight := p.ImageHeight / p.Threads
	workerEvents := make(chan Event)

	//TODO: Split image, send worker goroutines
	for t := 0; t < p.Threads; t++ {
		endY := (t + 1) * threadHeight
		if t == p.Threads-1 {
			endY = p.ImageHeight
		}
		workerParams := workerParams{
			StartX:      0,
			StartY:      t * threadHeight,
			EndX:        p.ImageWidth,
			EndY:        endY,
			ImageWidth:  p.ImageWidth,
			ImageHeight: p.ImageHeight,
			Turns:       p.Turns,
		}
		workerChannels := workerChannels{
			events: workerEvents,
		}
		//fmt.Println("Sent worker", t, "from", workerParams.StartY, "to", workerParams.EndY)
		go worker(world, workerParams, workerChannels)
	}

	isDone := false
	aliveCells := []util.Cell{}

	for {
		select {
		case event := <-workerEvents:
			switch e := event.(type) {
			case WorkerTurnComplete:
				workersCompletedTurn++
				if workersCompletedTurn == p.Threads {
					workersCompletedTurn = 0
					c.events <- TurnComplete{CompletedTurns: turn}
					turn++
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
		}
		if isDone {
			break
		}
	}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
