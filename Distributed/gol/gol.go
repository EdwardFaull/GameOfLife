package gol

import (
	"fmt"
	"net/rpc"
	"os"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

//Parameters to send
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type ClientParams struct {
	Turns          int
	Threads        int
	ImageWidth     int
	ImageHeight    int
	BrokerAddr     string
	ShouldContinue int
	Factories      int
	Testing        int
	TickFrequency  time.Duration
}

type controllerChannels struct {
	command  chan ioCommand
	ioIdle   chan bool
	input    chan uint8
	output   chan uint8
	filename chan string
}

type ReportType uint8

const (
	Ticking ReportType = iota
	Finished
)

func ClientToEngineParams(p ClientParams) Params {
	np := Params{
		Turns:       p.Turns,
		Threads:     p.Threads,
		ImageWidth:  p.ImageWidth,
		ImageHeight: p.ImageHeight,
	}
	return np
}

func Run(p ClientParams, events chan Event, keyPresses chan rune) {
	//Read image
	quit := make(chan bool)
	aliveCellsChan := make(chan []util.Cell, 10)
	engineParams := ClientToEngineParams(p)
	controllerChannels := makeIO(engineParams)
	aliveCells := readImage(engineParams, controllerChannels)
	mutex := &sync.Mutex{}

	if p.Testing == 0 {
		p.ShouldContinue = 0
		p.Factories = 2
		p.BrokerAddr = "192.168.0.2:8030"
		p.TickFrequency = 2000
	}
	//Dial broker address.
	client, err := rpc.Dial("tcp", (p.BrokerAddr))
	if err != nil {
		fmt.Println("Error: Client returned nil.", err)
		os.Exit(2)
	}
	status := new(StatusReport)
	towork := InitRequest{
		Alive:          aliveCells,
		Params:         engineParams,
		ShouldContinue: p.ShouldContinue,
		InboundIP:      util.GetOutboundIP(),
		Factories:      p.Factories,
	}
	//Call the broker
	client.Call(Initialise, towork, &status)
	go updateImage(events, aliveCellsChan, mutex)
	go ticker(engineParams, controllerChannels, p.TickFrequency, client, events, quit, aliveCellsChan, mutex)
	go keyboard(client, keyPresses, events, engineParams, controllerChannels, quit, aliveCellsChan, mutex)
}

//Read image in from folder
func readImage(p Params, c controllerChannels) []util.Cell {

	aliveCells := []util.Cell{}

	c.command <- ioCheckIdle
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

	c.command <- ioInput

	s := fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)
	c.filename <- s

	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			select {
			case b := <-c.input:
				if b == 255 {
					aliveCells = append(aliveCells, util.Cell{X: j, Y: i})
				}
			}
		}
	}

	return aliveCells
}

//Handles regular updates from engine.
func ticker(p Params, c controllerChannels, tickFrequency time.Duration, client *rpc.Client, events chan Event,
	quit chan bool, aliveCellsChan chan<- []util.Cell, mutex *sync.Mutex) {
	ticker := time.NewTicker(tickFrequency * time.Millisecond)
	isDone := false
	for {
		select {
		case <-ticker.C:
			aliveReport := TickReport{}
			fmt.Println("Ticking...")
			client.Call(Report, ReportRequest{InboundIP: util.GetOutboundIP()}, &aliveReport)
			switch aliveReport.ReportType {
			case Ticking:
				events <- AliveCellsCount{CompletedTurns: aliveReport.Turns, CellsCount: aliveReport.CellsCount}
				aliveCellsChan <- aliveReport.Alive
			case Finished:
				events <- FinalTurnComplete{CompletedTurns: aliveReport.Turns, Alive: aliveReport.Alive}
				events <- StateChange{aliveReport.Turns, Quitting, nil}
				outputImage(p, c, aliveReport.Alive, aliveReport.Turns)
				c.command <- ioCheckIdle
				<-c.ioIdle
				mutex.Lock()
				close(events)
				mutex.Unlock()
				isDone = true
			}
			if isDone {
				quit <- true
				return
			}
		case <-quit:
			ticker.Stop()
			return
		}
	}
}

//Handles keypresses from the user and calls the engine on keypresses
func keyboard(client *rpc.Client, keyPresses chan rune, events chan Event,
	p Params, c controllerChannels, quit chan bool, aliveCellsChan chan<- []util.Cell, mutex *sync.Mutex) {
	isDone := false
	for {
		select {
		case k := <-keyPresses:
			request := KeyPressRequest{Key: k, InboundIP: util.GetOutboundIP()}
			keyPressReport := KeyPressReport{Alive: nil, Turns: 0}
			client.Call(KeyPress, request, &keyPressReport)
			if keyPressReport.State != Saving {
				events <- StateChange{
					CompletedTurns: keyPressReport.Turns,
					Alive:          keyPressReport.Alive,
					NewState:       keyPressReport.State}
			}
			switch k {
			case 'p':
				aliveCellsChan <- keyPressReport.Alive
			case 's':
				outputImage(p, c, keyPressReport.Alive, keyPressReport.Turns)
			case 'q':
				outputImage(p, c, keyPressReport.Alive, keyPressReport.Turns)
				c.command <- ioCheckIdle
				<-c.ioIdle
				mutex.Lock()
				close(events)
				mutex.Unlock()
				isDone = true
				quit <- true
			case 'k':
				outputImage(p, c, keyPressReport.Alive, keyPressReport.Turns)
				c.command <- ioCheckIdle
				<-c.ioIdle
				mutex.Lock()
				close(events)
				mutex.Unlock()
				isDone = true
				quit <- true
			}
		case <-quit:
			isDone = true
		}
		if isDone {
			break
		}
	}
}

//Updates SDL view
func updateImage(events chan<- Event, aliveCellsChan <-chan []util.Cell, mutex *sync.Mutex) {
	previousAliveCells := []util.Cell{}
	for {
		newAliveCells := <-aliveCellsChan
		mutex.Lock()
		for _, cell := range previousAliveCells {
			events <- CellFlipped{CompletedTurns: 0, Cell: cell}
		}
		for _, cell := range newAliveCells {
			events <- CellFlipped{CompletedTurns: 0, Cell: cell}
		}
		events <- TurnComplete{CompletedTurns: 0}
		previousAliveCells = newAliveCells
		mutex.Unlock()
	}
}

//Save image to /out folder
func outputImage(p Params, c controllerChannels, aliveCells []util.Cell, turns int) {
	c.command <- ioOutput
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

//Starts the IO goroutine and returns a controllerChannels structure for later use.
func makeIO(engineParams Params) controllerChannels {
	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)
	input := make(chan byte)
	output := make(chan byte)
	filename := make(chan string)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: filename,
		output:   output,
		input:    input,
	}
	go startIo(engineParams, ioChannels)

	controllerChannels := controllerChannels{
		command:  ioCommand,
		ioIdle:   ioIdle,
		filename: filename,
		output:   output,
		input:    input,
	}
	return controllerChannels
}
