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
	ticker     chan bool
	aliveCells chan int
	turn       chan int
}

//Begin GoL execution
func publish(topic string, params stubs.PublishRequest, res *stubs.StatusReport, e *Engine) (err error) {
	alive, turns := gol.Run(params.Params.Params, params.Events, params.Keypresses, params.Params.Alive,
		e.ticker, e.aliveCells, e.turn)
	(*res).Alive = alive
	(*res).Turns = turns
	return err
}

func (e *Engine) Publish(req stubs.PublishRequest, res *stubs.StatusReport) (err error) {
	err = publish(req.Topic, req, res, e)
	return err
}

func (e *Engine) ReturnAlive(req stubs.PublishRequest, res *stubs.AliveReport) (err error) {
	fmt.Println("Returning alive cells")
	e.ticker <- true
	alive := <-e.aliveCells
	turn := <-e.turn
	(*res).Alive = alive
	(*res).Turn = turn
	fmt.Println("Received from distributor:", alive, turn)
	return err
}

// main is the function called when starting Game of Life with 'go run .'
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&Engine{make(chan gol.Event), make(chan bool), make(chan int), make(chan int)})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
