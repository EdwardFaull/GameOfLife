package main

import (
	"flag"
	"fmt"
	"net/rpc"
	"runtime"
	"time"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/sdl"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type controllerChannels struct {
	command  chan ioCommand
	ioIdle   chan bool
	input    chan uint8
	output   chan uint8
	filename chan string
}

func main() {
	runtime.LockOSThread()
	var params gol.Params

	flag.IntVar(
		&params.Threads,
		"t",
		8,
		"Specify the number of worker threads to use. Defaults to 8.")

	flag.IntVar(
		&params.ImageWidth,
		"w",
		512,
		"Specify the width of the image. Defaults to 512.")

	flag.IntVar(
		&params.ImageHeight,
		"h",
		512,
		"Specify the height of the image. Defaults to 512.")

	flag.IntVar(
		&params.Turns,
		"turns",
		10000000000,
		"Specify the number of turns to process. Defaults to 10000000000.")

	brokerAddr := flag.String("broker", "127.0.0.1:8030", "Address of broker instance")
	flag.Parse()

	//Read image

	events := make(chan gol.Event, 1000)
	keyPresses := make(chan rune, 10)
	quit := make(chan bool)

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
	go startIo(params, ioChannels)

	controllerChannels := controllerChannels{
		command:  ioCommand,
		ioIdle:   ioIdle,
		filename: filename,
		output:   output,
		input:    input,
	}
	aliveCells := readImage(params, controllerChannels)

	//Connect to RPC Server

	//Dial broker address.
	client, _ := rpc.Dial("tcp", *brokerAddr)
	status := new(stubs.StatusReport)
	initParams := stubs.InitParams{
		Alive:  aliveCells,
		Params: params,
	}
	towork := stubs.InitRequest{Params: &initParams}
	//Call the broker
	client.Call(stubs.Initialise, towork, &status)
	go ticker(client, events, quit)
	go keyboard(client, keyPresses, events, params, controllerChannels, quit)
	sdl.Start(params, events, keyPresses)
}

func readImage(p gol.Params, c controllerChannels) []util.Cell {

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

func ticker(client *rpc.Client, events chan gol.Event, quit <-chan bool) {
	ticker := time.NewTicker(2 * time.Second)
	isDone := false
	for {
		select {
		case <-ticker.C:
			aliveReport := stubs.TickReport{}
			client.Call(stubs.Report, stubs.ReportRequest{}, &aliveReport)
			if aliveReport.Alive != nil {
				events <- gol.FinalTurnComplete{CompletedTurns: aliveReport.Turns, Alive: aliveReport.Alive}
				isDone = true
			} else {
				events <- gol.AliveCellsCount{CompletedTurns: aliveReport.Turns, CellsCount: aliveReport.CellsCount}
			}
			if isDone {
				return
			}
		case <-quit:
			ticker.Stop()
			return
		}
	}
}

func keyboard(client *rpc.Client, keyPresses chan rune, events chan gol.Event,
	p gol.Params, c controllerChannels, quit chan<- bool) {
	previousAliveCells := []util.Cell{}
	isDone := false
	for {
		select {
		case k := <-keyPresses:
			fmt.Println("Received input: ", k)
			request := stubs.KeyPressRequest{Key: k}
			keyPressReport := stubs.KeyPressReport{Alive: nil, Turns: 0}
			client.Call(stubs.KeyPress, request, &keyPressReport)
			if keyPressReport.State != gol.Saving {
				events <- gol.StateChange{
					CompletedTurns: keyPressReport.Turns,
					Alive:          keyPressReport.Alive,
					NewState:       keyPressReport.State}
			}
			switch k {
			case 'p':
				for _, cell := range previousAliveCells {
					events <- gol.CellFlipped{CompletedTurns: keyPressReport.Turns, Cell: cell}
				}
				events <- gol.TurnComplete{CompletedTurns: 0}
				for _, cell := range keyPressReport.Alive {
					events <- gol.CellFlipped{CompletedTurns: keyPressReport.Turns, Cell: cell}
				}
				//events <- gol.TurnComplete{CompletedTurns: keyPressReport.Turns}
				previousAliveCells = keyPressReport.Alive
			case 's':
				outputImage(p, c, keyPressReport.Alive, keyPressReport.Turns)
			case 'q':
				outputImage(p, c, keyPressReport.Alive, keyPressReport.Turns)
				c.command <- ioCheckIdle
				<-c.ioIdle
				close(events)
				isDone = true
				quit <- true
			}
		}
		if isDone {
			break
		}
	}
}

func outputImage(p gol.Params, c controllerChannels, aliveCells []util.Cell, turns int) {
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
