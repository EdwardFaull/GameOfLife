package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"time"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/util"
)

type Factory struct {
	events             chan gol.Event
	keyPresses         chan rune
	keyPressEvents     chan gol.Event
	tickerChan         chan bool
	killChannel        chan bool
	killConfirmChannel chan bool
	fetchSignal        chan bool
	fetchLowerResponse chan gol.Filler
	fetchUpperResponse chan gol.Filler
	offset             int
	gameRunning        bool
}

func (f *Factory) Initialise(req gol.InitRequest, res *gol.StatusReport) (err error) {
	params := req.Params
	f.offset = req.StartY
	if params.ImageHeight < 12 {
		params.Threads = params.ImageHeight / 2
	} else {
		params.Threads = 12
	}
	if req.ShouldContinue == 0 {
		if f.gameRunning {
			f.killChannel <- true
			<-f.killConfirmChannel
			f.emptyChannels()
		}
		f.offset = req.StartY
		go gol.Distributor(params, req.Alive, f.events, f.keyPressEvents, f.keyPresses, f.tickerChan, f.killChannel, f.killConfirmChannel,
			req.LowerIP, req.UpperIP, f.fetchSignal, f.fetchLowerResponse, f.fetchUpperResponse, req.StartY)
		fmt.Println("Created new Distributor")
		f.gameRunning = true
	} else if req.ShouldContinue == 1 {
		if !f.gameRunning {
			fmt.Println("Error: no game running. Creating new game.")
			f.offset = req.StartY
			go gol.Distributor(params, req.Alive, f.events, f.keyPressEvents, f.keyPresses, f.tickerChan, f.killChannel, f.killConfirmChannel,
				req.LowerIP, req.UpperIP, f.fetchSignal, f.fetchLowerResponse, f.fetchUpperResponse, req.StartY)
			f.gameRunning = true
		} else {
			f.keyPresses <- 'r'
		}
	} else {
		fmt.Println("Incorrect flag value for continue. Must be either 0 or 1.")
	}
	return
}

func (f *Factory) emptyChannels() {
	for {
		if len(f.events) > 0 {
			<-f.events
		}
		if len(f.keyPresses) > 0 {
			<-f.keyPresses
		}
		if len(f.keyPressEvents) > 0 {
			<-f.keyPressEvents
		}
		if len(f.tickerChan) > 0 {
			<-f.tickerChan
		}
		if len(f.killChannel) > 0 {
			<-f.killChannel
		}
		if len(f.killConfirmChannel) > 0 {
			<-f.killConfirmChannel
		}
		if len(f.events) == 0 && len(f.keyPresses) == 0 &&
			len(f.keyPressEvents) == 0 && len(f.tickerChan) == 0 &&
			len(f.killChannel) == 0 && len(f.killConfirmChannel) == 0 {
			break
		}
	}
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
				(*res).Alive = offsetCells(f.offset, t.Alive)
				(*res).ReportType = gol.Ticking
				return err
			case gol.FinalTurnComplete:
				(*res).Alive = offsetCells(f.offset, t.Alive)
				(*res).Turns = t.CompletedTurns
				(*res).ReportType = gol.Finished
				f.gameRunning = false
				f.emptyChannels()
				return err
			}
		}
	}
}

func offsetCells(offset int, alive []util.Cell) []util.Cell {
	offsetAlive := []util.Cell{}
	for _, c := range alive {
		offsetAlive = append(offsetAlive, util.Cell{X: c.X, Y: c.Y + offset})
	}
	return offsetAlive
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
			fmt.Println("Got keypress event")
		}
	}
	return err
}

func (f *Factory) Kill(req gol.KillRequest, res *gol.StatusReport) (err error) {
	fmt.Println("Killing distributor")
	f.killChannel <- true
	fmt.Println("Sent killcode to distributor")
	<-f.killConfirmChannel
	fmt.Println("Killed distributor")
	go killFactory()
	return err
}

func (f *Factory) Fetch(req gol.FetchRequest, res *gol.FetchReport) (err error) {
	f.fetchSignal <- req.UpperOrLower
	var filler gol.Filler
	line := []byte{}
	if req.UpperOrLower {
		filler = <-f.fetchUpperResponse
		line = filler.GetUpperLine()
	} else {
		filler = <-f.fetchLowerResponse
		line = filler.GetLowerLine()
	}
	(*res).Line = line
	return err
}

func killFactory() {
	time.Sleep(1 * time.Second)
	os.Exit(1)
}

func main() {
	pAddr := flag.String("port", "8050", "Port to listen on")
	brokerAddr := flag.String("broker", "127.0.0.1:8030", "Address of broker instance")
	flag.Parse()
	client, _ := rpc.Dial("tcp", *brokerAddr)
	status := new(gol.StatusReport)
	rpc.Register(&Factory{make(chan gol.Event, 1000), make(chan rune, 10), make(chan gol.Event, 1000), make(chan bool, 10),
		make(chan bool, 1), make(chan bool, 1), make(chan bool, 10), make(chan gol.Filler, 10), make(chan gol.Filler, 10), 0, false})
	fmt.Println(*pAddr)
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Println(err)
	}
	client.Call(gol.Subscribe, gol.Subscription{FactoryAddress: util.GetOutboundIP() + ":" + *pAddr}, status)
	defer listener.Close()
	rpc.Accept(listener)
}
