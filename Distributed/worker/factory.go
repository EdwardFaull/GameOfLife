package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/util"
)

var (
	mulch = make(chan int, 2)
)

type Factory struct {
	events             chan gol.Event
	keyPresses         chan rune
	keyPressEvents     chan gol.Event
	tickerChan         chan bool
	killChannel        chan bool
	killConfirmChannel chan bool
	gameRunning        bool
}

func (f *Factory) Initialise(req gol.InitRequest, res *gol.StatusReport) (err error) {
	//todo worker functionality
	go gol.Distributor(req.Params.Params, req.Params.Alive, f.events, f.keyPressEvents, f.keyPresses, f.tickerChan, f.killChannel, f.killConfirmChannel)
	return
}

func (f *Factory) Report(req gol.ReportRequest, res *gol.TickReport) (err error) {
	f.tickerChan <- true
	for {
		select {
		case event := <-f.events:
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
				f.gameRunning = false
				//f.emptyChannels()
				return err
			}
		}
	}
}

func (f *Factory) KeyPress(req gol.KeyPressRequest, res *gol.KeyPressReport) (err error) {
	f.keyPresses <- req.Key
	select {
	case k := <-f.keyPressEvents:
		switch t := k.(type) {
		case gol.StateChange:
			(*res).Alive = t.Alive
			(*res).Turns = t.CompletedTurns
			(*res).State = t.NewState
		}
	}
	return err
}

func main() {
	pAddr := flag.String("port", "8050", "Port to listen on")
	brokerAddr := flag.String("broker", "127.0.0.1:8030", "Address of broker instance")
	flag.Parse()
	client, _ := rpc.Dial("tcp", *brokerAddr)
	status := new(gol.StatusReport)
	//client.Call(stubs.CreateChannel, stubs.ChannelRequest{Topic: "multiply", Buffer: 10}, status)
	//client.Call(stubs.CreateChannel, stubs.ChannelRequest{Topic: "divide", Buffer: 10}, status)
	rpc.Register(&Factory{make(chan gol.Event, 1000), make(chan rune, 10), make(chan gol.Event, 1000), make(chan bool, 10),
		make(chan bool, 1), make(chan bool, 1), false})
	fmt.Println(*pAddr)
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Println(err)
	}
	client.Call(gol.Subscribe, gol.Subscription{FactoryAddress: util.GetOutboundIP() + ":" + *pAddr, Callback: "Factory.Multiply"}, status)
	//client.Call(stubs.Subscribe, stubs.Subscription{Topic: "divide", FactoryAddress: getOutboundIP()+":"+*pAddr, Callback: "Factory.Divide"}, status)
	defer listener.Close()
	rpc.Accept(listener)
	flag.Parse()
}
