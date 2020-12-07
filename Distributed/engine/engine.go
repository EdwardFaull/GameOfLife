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
}

//Begin GoL execution
func (e *Engine) Initialise(req stubs.InitRequest, res *stubs.StatusReport) (err error) {
	params := req.Params
	fmt.Println("Init started")
	go gol.Run(params.Params, e.events, e.keyPresses, e.keyPressEvents, req.Params.Alive, e.tickerChan)
	fmt.Println("gol run set off")
	return err
}

func (e *Engine) Report(req stubs.ReportRequest, res *stubs.TickReport) (err error) {
	e.tickerChan <- true
	for {
		select {
		case event := <-e.events:
			switch t := event.(type) {
			case gol.AliveCellsCount:
				fmt.Println("AliveCellsCount: Alive:", t.CellsCount, "   Turn:", t.CompletedTurns)
				(*res).CellsCount = t.CellsCount
				(*res).Turns = t.CompletedTurns
				return err
			case gol.FinalTurnComplete:
				fmt.Println("FinalTurnComplete: Alive:", t.Alive, "   Turn:", t.CompletedTurns)
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
		time.NewTicker(2 * time.Second)})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
