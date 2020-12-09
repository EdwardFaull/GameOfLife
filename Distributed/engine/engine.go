package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"time"

	"uk.ac.bris.cs/gameoflife/gol"
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
func (e *Engine) Initialise(req gol.InitRequest, res *gol.StatusReport) (err error) {
	params := req.Params
	if req.ShouldContinue == 0 {
		if e.gameRunning {
			e.killChannel <- true
			//TODO: Block until game destroyed better
			time.Sleep(1 * time.Second)
			e.emptyChannels()
		}
		go gol.Distributor(params.Params, req.Params.Alive, e.events, e.keyPressEvents, e.keyPresses, e.tickerChan, e.killChannel)
		e.gameRunning = true
	} else if req.ShouldContinue == 1 {
		if !e.gameRunning {
			fmt.Println("Error: no game running. Creating new game.")
			go gol.Distributor(params.Params, req.Params.Alive, e.events, e.keyPressEvents, e.keyPresses, e.tickerChan, e.killChannel)
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
		if len(e.killChannel) > 0 {
			<-e.killChannel
		}
		if len(e.events) == 0 && len(e.keyPresses) == 0 && len(e.keyPressEvents) == 0 && len(e.tickerChan) == 0 && len(e.killChannel) == 0 {
			break
		}
	}
}

func (e *Engine) Report(req gol.ReportRequest, res *gol.TickReport) (err error) {
	e.tickerChan <- true
	for {
		select {
		case event := <-e.events:
			switch t := event.(type) {
			case gol.AliveCellsCount:
				(*res).CellsCount = t.CellsCount
				(*res).Turns = t.CompletedTurns
				(*res).ReportType = gol.Ticking
				return err
			case gol.FinalTurnComplete:
				(*res).Alive = t.Alive
				(*res).Turns = t.CompletedTurns
				(*res).ReportType = gol.Finished
				return err
			}
		}
	}
	return err
}

func (e *Engine) KeyPress(req gol.KeyPressRequest, res *gol.KeyPressReport) (err error) {
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
