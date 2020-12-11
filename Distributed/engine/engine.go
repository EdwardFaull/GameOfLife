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

type Engine struct {
	factoryChannels map[string](chan gol.Request)
	factoriesBusy   map[string]bool
	IPLinks         map[string][]string
	ReportChans     map[string](chan gol.BaseReport)
}

//Gets as many free workers as possible (up to the limit) for the controller to use
func findFreeWorker(limit int, factoriesBusy map[string]bool) []string {
	factories := []string{}
	for k, v := range factoriesBusy {
		if !v {
			factoriesBusy[k] = true
			factories = append(factories, k)
			if len(factories) == limit {
				return factories
			}
		}
	}
	return factories
}

//Begin GoL execution
func (e *Engine) Initialise(req gol.InitRequest, res *gol.StatusReport) (err error) {
	//Allocate factory to controller
	factoryIPs := []string{}
	//If the controller doesn't already have IP addresses assigned to it, or more IP addresses are available, re-calculate.
	if _, ok := e.IPLinks[req.InboundIP]; !ok {
		e.IPLinks[req.InboundIP] = findFreeWorker(req.Factories, e.factoriesBusy)
	} else if len(e.IPLinks[req.InboundIP]) < req.Factories {
		e.IPLinks[req.InboundIP] = findFreeWorker(req.Factories, e.factoriesBusy)
	}
	factoryIPs = e.IPLinks[req.InboundIP]
	if len(factoryIPs) == 0 {
		fmt.Println("Error: No free factories")
		return err
	}
	e.IPLinks[req.InboundIP] = factoryIPs
	req.Factories = len(factoryIPs)
	//Initialise factories
	for i, factoryIP := range factoryIPs {
		offset := 0
		if i == req.Factories-1 {
			offset = req.Params.ImageHeight % req.Factories
		}
		lowerIP := ""
		if i == 0 {
			lowerIP = factoryIPs[req.Factories-1]
		} else {
			lowerIP = factoryIPs[i-1]
		}
		upperIP := factoryIPs[(i+1)%req.Factories]

		parameters := gol.Params{
			ImageWidth:  req.Params.ImageWidth,
			ImageHeight: req.Params.ImageHeight/req.Factories + offset,
			Turns:       req.Params.Turns,
			Threads:     req.Params.Threads,
		}

		//Select only the alive cells needed for this factory's section of world
		alive := []util.Cell{}
		for _, c := range req.Alive {
			if i*(req.Params.ImageHeight/req.Factories) <= c.Y && c.Y < (i+1)*(req.Params.ImageHeight/req.Factories) {
				alive = append(alive, c)
			}
		}
		factoryRequest := gol.InitRequest{
			Params:         parameters,
			ShouldContinue: req.ShouldContinue,
			InboundIP:      req.InboundIP,
			Factories:      req.Factories,
			Alive:          alive,
			UpperIP:        upperIP,
			LowerIP:        lowerIP,
			StartY:         i * (req.Params.ImageHeight / req.Factories),
		}
		e.factoryChannels[factoryIP] <- factoryRequest
	}
	return err
}

//Handles requests to a factory, then sends their reports into its linked report channel
func (e *Engine) subscriberLoop(client *rpc.Client, addr chan gol.Request, factoryAddress string) {
	for {
		job := <-addr
		var err error
		switch j := job.(type) {
		case gol.InitRequest:
			response := gol.StatusReport{}
			err = client.Call("Factory.Initialise", j, &response)
		case gol.ReportRequest:
			response := gol.TickReport{}
			err = client.Call("Factory.Report", j, &response)
			e.ReportChans[factoryAddress] <- response
		case gol.KeyPressRequest:
			response := gol.KeyPressReport{}
			err = client.Call("Factory.KeyPress", j, &response)
			e.ReportChans[factoryAddress] <- response
		case gol.KillRequest:
			response := gol.StatusReport{}
			err = client.Call(gol.Kill, j, &response)
		}
		if err != nil {
			fmt.Println("Error")
			fmt.Println(err)
			fmt.Println("Closing subscriber thread.")
			break
		}
	}
}

//Establishes the RPC connection between a factory and the engine
func (e *Engine) subscribe(factoryAddress string) (err error) {
	client, err := rpc.Dial("tcp", factoryAddress)
	if err == nil {
		e.factoriesBusy[factoryAddress] = false
		e.factoryChannels[factoryAddress] = make(chan gol.Request, 10)
		e.ReportChans[factoryAddress] = make(chan gol.BaseReport, 10)
		fmt.Println(factoryAddress, "subscribed to engine.")
		go e.subscriberLoop(client, e.factoryChannels[factoryAddress], factoryAddress)
	} else {
		fmt.Println("Error subscribing ", factoryAddress)
		fmt.Println(err)
		return err
	}
	return
}

//Subscribe is called by the factory on creation, and creates a subscriber loop in engine that
//handles requests by its linked controller
func (e *Engine) Subscribe(req gol.Subscription, res *gol.StatusReport) (err error) {
	err = e.subscribe(req.FactoryAddress)
	if err != nil {
		fmt.Println("Error during creation of subscription. IP =", req.FactoryAddress)
	}
	return err
}

//Report is called by the controller on each tick, and sends a request to all linked subscribers.
//If the subscribers have finished executing their Game of Life, they return with reportType = Finished
func (e *Engine) Report(req gol.ReportRequest, res *gol.TickReport) (err error) {
	inboundIP := req.InboundIP
	factoryIPs := e.IPLinks[inboundIP]
	(*res).CellsCount = 0
	(*res).Alive = []util.Cell{}
	for _, ip := range factoryIPs {
		e.factoryChannels[ip] <- req
		report := <-e.ReportChans[ip]
		switch r := report.(type) {
		case gol.TickReport:
			//Add information together to get complete world
			(*res).Turns = r.Turns
			(*res).Alive = append((*res).Alive, r.Alive...)
			(*res).CellsCount += r.CellsCount
			(*res).ReportType = r.ReportType
			if r.ReportType == gol.Finished {
				//Mark factories as not busy so they can be reused
				e.factoriesBusy[ip] = false
			}
			break
		default:
			e.ReportChans[ip] <- report
		}
	}
	return err
}

//KeyPress is called by the controller when a key is pressed in SDL. It sends a request to
//subscribing factories, which then execute their keypress function.
func (e *Engine) KeyPress(req gol.KeyPressRequest, res *gol.KeyPressReport) (err error) {
	inboundIP := req.InboundIP
	factoryIPs := e.IPLinks[inboundIP]
	for _, factoryIP := range factoryIPs {
		e.factoryChannels[factoryIP] <- req
		report := <-e.ReportChans[factoryIP]
		switch r := report.(type) {
		case gol.KeyPressReport:
			(*res).Alive = append((*res).Alive, r.Alive...)
			(*res).Turns = r.Turns
			(*res).State = r.State
			break
		default:
			e.ReportChans[factoryIP] <- report
		}
	}
	if req.Key == 'k' {
		//If k is pressed, send a request to cleanly close all factories
		for _, ch := range e.factoryChannels {
			ch <- gol.KillRequest{}
		}
		go killEngine()
	}
	return err
}

//Cleanly closes engine, leaving time for the KeyPress function to return to the controller
func killEngine() {
	time.Sleep(1 * time.Second)
	os.Exit(1)
}

// main is the function called when starting Game of Life with 'go run .'
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&Engine{make(map[string]chan gol.Request), make(map[string]bool), make(map[string][]string), make(map[string]chan gol.BaseReport)})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
