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
	IPAddresses map[string](chan gol.Request)
	workersBusy map[string]bool
	IPLinks     map[string][]string
	ReportChans map[string](chan gol.BaseReport)
}

func (e *Engine) FindFreeWorker(limit int) []string {
	workers := []string{}
	for k, v := range e.workersBusy {
		fmt.Println("k = ", k, "v = ", v)
		if !v {
			e.workersBusy[k] = true
			workers = append(workers, k)
		}
	}
	return workers
}

//Begin GoL execution
func (e *Engine) Initialise(req gol.InitRequest, res *gol.StatusReport) (err error) {
	workerIPs := []string{}
	if _, OK := e.IPLinks[req.InboundIP]; !OK {
		workerIPs = e.FindFreeWorker(req.Workers)
		e.IPLinks[req.InboundIP] = workerIPs
	}
	workerIPs = e.IPLinks[req.InboundIP]
	req.Workers = len(workerIPs)
	for i, workerIP := range workerIPs {
		offset := 0
		if i == req.Workers-1 {
			offset = req.Params.ImageHeight % req.Workers
		}
		lowerIP := ""
		if i == 0 {
			lowerIP = workerIPs[req.Workers-1]
		} else {
			lowerIP = workerIPs[i-1]
		}
		upperIP := workerIPs[(i+1)%req.Workers]

		parameters := gol.Params{
			ImageWidth:  req.Params.ImageWidth,
			ImageHeight: req.Params.ImageHeight/req.Workers + offset,
			Turns:       req.Params.Turns,
			Threads:     req.Params.Threads,
		}
		alive := []util.Cell{}

		for _, c := range req.Alive {
			if i*(req.Params.ImageHeight/req.Workers) <= c.Y && c.Y < (i+1)*(req.Params.ImageHeight/req.Workers) {
				alive = append(alive, c)
			}
		}
		workerRequest := gol.InitRequest{
			Params:         parameters,
			ShouldContinue: req.ShouldContinue,
			InboundIP:      req.InboundIP,
			Workers:        req.Workers,
			Alive:          alive,
			UpperIP:        upperIP,
			LowerIP:        lowerIP,
			StartY:         i * (req.Params.ImageHeight / req.Workers),
		}
		e.IPAddresses[workerIP] <- workerRequest
	}
	return err
}

func (e *Engine) subscriber_loop(client *rpc.Client, addr chan gol.Request, factoryAddr string) {
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
			e.ReportChans[factoryAddr] <- response
		case gol.KeyPressRequest:
			response := gol.KeyPressReport{}
			err = client.Call("Factory.KeyPress", j, &response)
			e.ReportChans[factoryAddr] <- response
		case gol.KillRequest:
			response := gol.StatusReport{}
			err = client.Call(gol.Kill, j, &response)
		}
		if err != nil {
			fmt.Println("Error")
			fmt.Println(err)
			fmt.Println("Closing subscriber thread.")
			//Place the unfulfilled job back on the topic channel.
			break
		}
	}
}

//The subscribe function registers a worker to the topic, creating an RPC client,
//and will use the given callback string as the callback function whenever work
//is available.
func (e *Engine) subscribe(factoryAddress string) (err error) {
	client, err := rpc.Dial("tcp", factoryAddress)
	if err == nil {
		e.workersBusy[factoryAddress] = false
		e.IPAddresses[factoryAddress] = make(chan gol.Request, 10)
		e.ReportChans[factoryAddress] = make(chan gol.BaseReport, 10)
		fmt.Println(factoryAddress, "subscribed to engine.")
		go e.subscriber_loop(client, e.IPAddresses[factoryAddress], factoryAddress)
	} else {
		fmt.Println("Error subscribing ", factoryAddress)
		fmt.Println(err)
		return err
	}
	return
}

func (e *Engine) Subscribe(req gol.Subscription, res *gol.StatusReport) (err error) {
	err = e.subscribe(req.FactoryAddress)
	if err != nil {
		//res.Message = "Error during subscription"
	}
	return err
}

func (e *Engine) Report(req gol.ReportRequest, res *gol.TickReport) (err error) {
	inboundIP := req.InboundIP
	workerIPs := e.IPLinks[inboundIP]
	(*res).CellsCount = 0
	(*res).Alive = []util.Cell{}
	for _, ip := range workerIPs {
		e.IPAddresses[ip] <- req
		report := <-e.ReportChans[ip]
		switch r := report.(type) {
		case gol.TickReport:
			(*res).Turns = r.Turns
			(*res).Alive = append((*res).Alive, r.Alive...)
			(*res).CellsCount += r.CellsCount
			(*res).ReportType = r.ReportType
			if r.ReportType == gol.Finished {
				e.workersBusy[ip] = false
			}
			break
		default:
			e.ReportChans[ip] <- report
		}
	}

	return err
}

func (e *Engine) Kill(req gol.KillRequest, res *gol.StatusReport) (err error) {
	os.Exit(1)
	return err
}

func (e *Engine) KeyPress(req gol.KeyPressRequest, res *gol.KeyPressReport) (err error) {
	inboundIP := req.InboundIP
	workerIPs := e.IPLinks[inboundIP]
	for _, workerIP := range workerIPs {
		e.IPAddresses[workerIP] <- req
		report := <-e.ReportChans[workerIP]
		switch r := report.(type) {
		case gol.KeyPressReport:
			(*res).Alive = append((*res).Alive, r.Alive...)
			(*res).Turns = r.Turns
			(*res).State = r.State
			break
		default:
			e.ReportChans[workerIP] <- report
		}
	}
	if req.Key == 'k' {
		for _, ch := range e.IPAddresses {
			ch <- gol.KillRequest{}
		}
		go killEngine()
	}
	return err
}

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
