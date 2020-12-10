package gol

import (
	"fmt"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events         chan<- Event
	keyPressEvents chan<- Event
	keyPresses     <-chan rune

	workerEvents         chan Event
	workerKeyPresses     []chan rune
	fillers              []chan Filler
	globalFiller         chan Filler
	turnFinishedChannels []chan int
	ticker               <-chan bool
	killChan             <-chan bool
	killConfirmChan      chan<- bool
	workerKillChan       []chan bool
	fetchSignal          chan bool
	fetchLowerResponse   chan<- Filler
	fetchUpperResponse   chan<- Filler
}

// distributor divides the work between workers and interacts with other goroutines.
func Distributor(p Params, alive []util.Cell, events chan Event, keyPressEvents chan Event, keyPresses chan rune,
	ticker chan bool, killChan <-chan bool, killConfirmChan chan<- bool, lowerIP string, upperIP string,
	fetchSignal chan bool, fetchLowerResponse chan<- Filler, fetchUpperResponse chan<- Filler, startY int) ([]util.Cell, int) {
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}
	for _, c := range alive {
		world[c.Y-startY][c.X] = 255
	}
	lowerClient, err := rpc.Dial("tcp", lowerIP)
	upperClient, err := rpc.Dial("tcp", upperIP)
	if err != nil {
		fmt.Println("Error:", err)
	}

	lowerClientFetchRequest := make(chan bool, 10)
	lowerClientFetchLine := make(chan []byte, 10)
	go fetchDistributedFillers(lowerClient, lowerClientFetchRequest, lowerClientFetchLine, false)

	upperClientFetchRequest := make(chan bool, 10)
	upperClientFetchLine := make(chan []byte, 10)
	go fetchDistributedFillers(upperClient, upperClientFetchRequest, upperClientFetchLine, true)

	workerEvents := make(chan Event, 1000)
	globalFiller := make(chan Filler, 10)

	c := distributorChannels{
		events:               events,
		keyPressEvents:       keyPressEvents,
		keyPresses:           keyPresses,
		workerEvents:         workerEvents,
		workerKeyPresses:     make([]chan rune, p.Threads),
		fillers:              make([]chan Filler, p.Threads),
		globalFiller:         globalFiller,
		turnFinishedChannels: make([]chan int, p.Threads),
		ticker:               ticker,
		killChan:             killChan,
		killConfirmChan:      killConfirmChan,
		workerKillChan:       make([]chan bool, p.Threads),
		fetchSignal:          fetchSignal,
		fetchLowerResponse:   fetchLowerResponse,
		fetchUpperResponse:   fetchUpperResponse,
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

		fillerElement := make(chan Filler, 10)
		c.fillers[t] = fillerElement

		finishedChannel := make(chan int)
		c.turnFinishedChannels[t] = finishedChannel

		keyPress := make(chan rune, 10)
		c.workerKeyPresses[t] = keyPress

		killChan := make(chan bool, 1)
		c.workerKillChan[t] = killChan

		var fetchRequest chan bool
		var fetchResponse chan []byte
		if t == 0 {
			fetchRequest = lowerClientFetchRequest
			fetchResponse = lowerClientFetchLine
		} else if t == p.Threads-1 {
			fetchRequest = upperClientFetchRequest
			fetchResponse = upperClientFetchLine
		}

		workerParams := workerParams{
			StartY:      startY,
			EndY:        endY,
			ImageWidth:  p.ImageWidth,
			ImageHeight: endY - startY,
			Turns:       p.Turns,
			Threads:     p.Threads,
		}
		workerChannels := workerChannels{
			events:          workerEvents,
			globalFiller:    globalFiller,
			workerFiller:    fillerElement,
			finishedChannel: finishedChannel,
			keyPresses:      keyPress,
			killChan:        killChan,
			fetchRequest:    fetchRequest,
			fetchResponse:   fetchResponse,
		}
		go worker(world[startY:endY], workerParams, workerChannels, t)
	}

	aliveCells, turn := handleChannels(p, c)

	return aliveCells, turn
}

func sendLinesToWorker(p Params, f Filler, c distributorChannels) {
	worker := f.workerID
	upperWorker := (worker + 1) % p.Threads
	lowerWorker := (worker - 1)
	if lowerWorker < 0 {
		lowerWorker = lowerWorker + p.Threads
	}

	if worker == 0 || worker == p.Threads-1 {
		for {
			b := <-c.fetchSignal
			if b && worker == p.Threads-1 {
				filler := Filler{lowerLine: f.upperLine, upperLine: f.upperLine, workerID: worker}
				c.fetchLowerResponse <- filler
				c.fillers[lowerWorker] <- Filler{lowerLine: f.lowerLine, upperLine: nil, workerID: worker}
				break
			} else if !b && worker == 0 {
				filler := Filler{lowerLine: f.lowerLine, upperLine: f.lowerLine, workerID: worker}
				c.fetchUpperResponse <- filler
				c.fillers[upperWorker] <- Filler{lowerLine: nil, upperLine: f.upperLine, workerID: worker}
				break
			} else {
				c.fetchSignal <- b
			}
		}
	} else {
		c.fillers[lowerWorker] <- Filler{lowerLine: f.lowerLine, upperLine: nil, workerID: worker}
		c.fillers[upperWorker] <- Filler{lowerLine: nil, upperLine: f.upperLine, workerID: worker}
	}
}

func fetchDistributedFillers(client *rpc.Client, fetchRequest <-chan bool, fetchLine chan<- []byte, upperOrLower bool) {
	for {
		<-fetchRequest
		report := FetchReport{}
		client.Call(Fetch, FetchRequest{UpperOrLower: upperOrLower}, &report)
		fetchLine <- report.Line
	}
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
					//Send all clear to workers to start next turn
					fmt.Println("Turn Complete")
					turn++
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
			case 'k':
				c.keyPressEvents <- StateChange{turn, Quitting, prevTurnAliveCells}
			}
		case <-c.ticker:
			c.events <- AliveCellsCount{CompletedTurns: turn, CellsCount: len(prevTurnAliveCells), Alive: prevTurnAliveCells}
		case <-c.killChan:
			for _, e := range c.workerKillChan {
				e <- true
			}
			c.killConfirmChan <- true
			return aliveCells, turn
		}
		if isDone {
			return aliveCells, turn
		}
	}
}
