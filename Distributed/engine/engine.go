package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"time"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/stubs"
)

type Engine struct {
	events         chan gol.Event
	keyPresses     chan rune
	keyPressEvents chan gol.Event
	tickerChan     chan bool
	ticker         *time.Ticker
	killChannel    chan bool
	gameRunning    bool
}

//Begin GoL execution
func (e *Engine) Initialise(req stubs.InitRequest, res *stubs.StatusReport) (err error) {
	params := req.Params
	if req.ShouldContinue == 0 {
		if e.gameRunning {
			e.killChannel <- true
			e.emptyChannels()
		}
		go gol.Run(params.Params, e.events, e.keyPresses, e.keyPressEvents, req.Params.Alive, e.tickerChan, e.killChannel)
		e.gameRunning = true
	} else if req.ShouldContinue == 1 {
		if !e.gameRunning {
			fmt.Println("Error: no game running. Creating new game.")
			go gol.Run(params.Params, e.events, e.keyPresses, e.keyPressEvents, req.Params.Alive, e.tickerChan, e.killChannel)
			e.gameRunning = true
		} else {
			e.keyPresses <- 'r'
		}
	} else {
		fmt.Println("Incorrect flag value for continue. Must be either 0 or 1.")
	}
	return err
}

func (e *Engine) emptyChannels() {
	for {
		fmt.Println("Emptying channels....")
		if len(e.events) > 0 {
			<-e.events
		}
		if len(e.keyPresses) > 0 {
			<-e.keyPresses
		}
		if len(e.keyPressEvents) > 0 {
			<-e.keyPressEvents
		}
		if len(e.tickerChan) > 0 {
			<-e.tickerChan
		}
		if len(e.events) == 0 && len(e.keyPresses) == 0 && len(e.keyPressEvents) == 0 && len(e.tickerChan) == 0 {
			break
		}
	}
}

func (e *Engine) Report(req stubs.ReportRequest, res *stubs.TickReport) (err error) {
	e.tickerChan <- true
	for {
		select {
		case event := <-e.events:
			switch t := event.(type) {
			case gol.AliveCellsCount:
				//fmt.Println("AliveCellsCount: Alive:", t.CellsCount, "   Turn:", t.CompletedTurns)
				(*res).CellsCount = t.CellsCount
				(*res).Turns = t.CompletedTurns
				return err
			case gol.FinalTurnComplete:
				//fmt.Println("FinalTurnComplete: Alive:", t.Alive, "   Turn:", t.CompletedTurns)
				(*res).Alive = t.Alive
				(*res).Turns = t.CompletedTurns
				return err
			}
		}
	}
	return err
}

func (e *Engine) KeyPress(req stubs.KeyPressRequest, res *stubs.KeyPressReport) (err error) {
	fmt.Println("Doing KeyPress")
	e.keyPresses <- req.Key
	select {
	case k := <-e.keyPressEvents:
		switch t := k.(type) {
		case gol.StateChange:
			(*res).Alive = t.Alive
			(*res).Turns = t.CompletedTurns
			(*res).State = t.NewState
		}
	}
	return err
}

// main is the function called when starting Game of Life with 'go run .'
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&Engine{make(chan gol.Event, 1000), make(chan rune, 10), make(chan gol.Event, 1000), make(chan bool, 10),
		time.NewTicker(2 * time.Second), make(chan bool, 1), false})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
