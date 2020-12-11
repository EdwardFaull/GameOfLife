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

//Initialise creates a new distributor to handle the incoming request.
//If the shouldContinue flag is up, it will resume whatever GoL it is already running.
func (f *Factory) Initialise(req gol.InitRequest, res *gol.StatusReport) (err error) {
	params := req.Params
	f.offset = req.StartY
	//Hard-codes the number of threads to be 12, unless if there's not enough space.
	if params.ImageHeight < 12 {
		params.Threads = params.ImageHeight / 2
	} else {
		params.Threads = 12
	}
	//If req doesn't want to continue an existing game, empties all channels
	if req.ShouldContinue == 0 {
		if f.gameRunning {
			f.killChannel <- true
			<-f.killConfirmChannel
			f.emptyChannels()
		}
		f.offset = req.StartY
		go gol.Distributor(params, req.Alive, f.events, f.keyPressEvents, f.keyPresses, f.tickerChan, f.killChannel, f.killConfirmChannel,
			req.LowerIP, req.UpperIP, f.fetchSignal, f.fetchLowerResponse, f.fetchUpperResponse, req.StartY)
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

//emptyChannels removes all items from the factory's channels.
//Called before beginning a new GoL to ensure nothing is left over from before.
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

//Report handles ReportRequests sent to the factory. It asks the distributor for an update,
//and when sent back updates the TickReport and returns.
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

//offsetCells adds a Y-axis offset to each cell passed into it, in order to
//make the values correct depending on which factory is operating.
func offsetCells(offset int, alive []util.Cell) []util.Cell {
	offsetAlive := []util.Cell{}
	for _, c := range alive {
		offsetAlive = append(offsetAlive, util.Cell{X: c.X, Y: c.Y + offset})
	}
	return offsetAlive
}

//KeyPress handles KeyPressRequests sent to the factory.
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

//Kill shuts down the distributor and then cleanly closes the factory.
func (f *Factory) Kill(req gol.KillRequest, res *gol.StatusReport) (err error) {
	f.killChannel <- true
	<-f.killConfirmChannel
	go killFactory()
	return err
}

//Fetch is called by neighbouring factories. It receives a filler line from the distributor,
//and then sends it back to its neighbour. The line returned depends on the value of req.UpperOrLower.
//If = true, it sends the upper line, if = false, it sends the lower line.
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

//Cleanly closes the program.
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
		make(chan bool, 1), make(chan bool, 1), make(chan bool, 10), make(chan gol.Filler, 10),
		make(chan gol.Filler, 10), 0, false})
	fmt.Println(*pAddr)
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Println(err)
	}
	client.Call(gol.Subscribe, gol.Subscription{FactoryAddress: util.GetOutboundIP() + ":" + *pAddr}, status)
	defer listener.Close()
	rpc.Accept(listener)
}
