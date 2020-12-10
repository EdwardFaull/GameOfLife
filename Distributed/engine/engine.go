package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/gol"
)

type Engine struct {
	IPAddresses map[string](chan gol.Request)
	workersBusy map[string]bool
	IPLinks     map[string]string
	ReportChans map[string](chan gol.BaseReport)
}

func (e *Engine) FindFreeWorker() string {
	for k, v := range e.workersBusy {
		fmt.Println("k = ", k, "v = ", v)
		if !v {
			return k
		}
	}
	return ""
}

//Begin GoL execution
func (e *Engine) Initialise(req gol.InitRequest, res *gol.StatusReport) (err error) {
	fmt.Println("Entered Initialise")
	if _, OK := e.IPLinks[req.InboundIP]; !OK {
		fmt.Println("Finding free worker...")
		e.IPLinks[req.InboundIP] = e.FindFreeWorker()
	}
	workerIP := e.IPLinks[req.InboundIP]
	fmt.Println("Found worker. Sending req...")
	fmt.Println("Worker ID: ", workerIP)
	e.IPAddresses[workerIP] <- req
	fmt.Println("Sent req")
	e.workersBusy[workerIP] = true
	return err
}

func (e *Engine) subscriber_loop(client *rpc.Client, addr chan gol.Request, factoryAddr string) {
	for {
		job := <-addr
		var err error
		switch j := job.(type) {
		case gol.InitRequest:
			response := new(gol.StatusReport)
			err = client.Call("Factory.Initialise", j, &response)
		case gol.ReportRequest:
			response := gol.TickReport{}
			err = client.Call("Factory.Report", j, &response)
			e.ReportChans[factoryAddr] <- response
		case gol.KeyPressRequest:
			response := gol.KeyPressReport{}
			err = client.Call("Factory.KeyPress", j, &response)
			e.ReportChans[factoryAddr] <- response
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
	workerIP := e.IPLinks[inboundIP]
	e.IPAddresses[workerIP] <- req
	report := <-e.ReportChans[workerIP]
	switch r := report.(type) {
	case gol.TickReport:
		fmt.Println("Completed turns = ", r.Turns)
		(*res).Turns = r.Turns
		(*res).Alive = r.Alive
		(*res).CellsCount = r.CellsCount
		(*res).ReportType = r.ReportType
		if r.ReportType == gol.Finished {
			workerIP := e.IPLinks[req.InboundIP]
			e.workersBusy[workerIP] = false
		}
		break
	default:
		e.ReportChans[workerIP] <- report
	}
	return err
}

func (e *Engine) KeyPress(req gol.KeyPressRequest, res *gol.KeyPressReport) (err error) {
	inboundIP := req.InboundIP
	workerIP := e.IPLinks[inboundIP]
	e.IPAddresses[workerIP] <- req
	report := <-e.ReportChans[workerIP]
	fmt.Println("Received report from factory")
	switch r := report.(type) {
	case gol.KeyPressReport:
		fmt.Println("Completed turns = ", r.Turns)
		(*res).Alive = r.Alive
		(*res).Turns = r.Turns
		(*res).State = r.State
		break
	default:
		e.ReportChans[workerIP] <- report
	}
	return err
}

// main is the function called when starting Game of Life with 'go run .'
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&Engine{make(map[string]chan gol.Request), make(map[string]bool), make(map[string]string), make(map[string]chan gol.BaseReport)})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
