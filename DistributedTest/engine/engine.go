package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/stubs"
)

type Engine struct {
	events     chan gol.Event
	keyPresses chan rune
	ticker     chan bool
}

//Begin GoL execution
func (e *Engine) Initialise(req stubs.InitRequest, res *stubs.StatusReport) (err error) {
	params := req.Params
	fmt.Println("Init started")
	go gol.Run(params.Params, e.events, e.keyPresses, req.Params.Alive, e.ticker)
	fmt.Println("gol run set off")
	return err
}

func (e *Engine) report(req stubs.ReportRequest, res *stubs.StatusReport) {
	endLoop := false
	for {
		select {
		case event := <-e.events:
			switch t := event.(type) {
			case gol.TurnComplete:
				//TODO
			case gol.FinalTurnComplete:
				(*res).Alive = t.Alive
				(*res).Turns = t.CompletedTurns
				endLoop = true
			default:
				e.events <- event
			}
		}
		if endLoop == true {
			break
		}
	}
}

func (e *Engine) Report(req stubs.ReportRequest, res *stubs.StatusReport) (err error) {
	go e.report(req, res)
	return err
}

func (e *Engine) Tick(req stubs.TickRequest, res *stubs.StatusReport) (err error) {
	endLoop := false
	fmt.Println("Received TickRequest")
	e.ticker <- true
	fmt.Println("Sent ticker bool")
	for {
		fmt.Println("Entered for loop")
		select {
		case event := <-e.events:
			fmt.Println("Received event")
			switch t := event.(type) {
			case gol.AliveCellsCount:
				fmt.Println("Received AliveCellsCount event")
				(*res).Turns = t.CompletedTurns
				(*res).Alive = nil
				endLoop = true
			default:
				fmt.Println("Received default event")
				e.events <- event
			}
		}
		if endLoop == true {
			break
		}
	}
	return err
}

func (e *Engine) KeyPress(req stubs.KeyPressRequest, res *stubs.StatusReport) (err error) {

	return err
}

// main is the function called when starting Game of Life with 'go run .'
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&Engine{make(chan gol.Event, 1000), make(chan rune, 10), make(chan bool, 10)})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
