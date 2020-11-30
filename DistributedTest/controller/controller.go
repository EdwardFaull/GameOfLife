package main

import (
	"flag"
	"fmt"
	"net/rpc"
	"runtime"
	"time"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type controllerChannels struct {
	command  chan ioCommand
	ioIdle   chan bool
	input    chan uint8
	output   chan uint8
	filename chan string
}

func main() {
	runtime.LockOSThread()
	var params gol.Params

	flag.IntVar(
		&params.Threads,
		"t",
		8,
		"Specify the number of worker threads to use. Defaults to 8.")

	flag.IntVar(
		&params.ImageWidth,
		"w",
		512,
		"Specify the width of the image. Defaults to 512.")

	flag.IntVar(
		&params.ImageHeight,
		"h",
		512,
		"Specify the height of the image. Defaults to 512.")

	flag.IntVar(
		&params.Turns,
		"turns",
		10000000000,
		"Specify the number of turns to process. Defaults to 10000000000.")

	brokerAddr := flag.String("broker", "127.0.0.1:8030", "Address of broker instance")
	topic := "gol"
	flag.Parse()

	//Read image

	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)
	input := make(chan byte)
	output := make(chan byte)
	filename := make(chan string)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: filename,
		output:   output,
		input:    input,
	}
	go startIo(params, ioChannels)

	controllerChannels := controllerChannels{
		command:  ioCommand,
		ioIdle:   ioIdle,
		filename: filename,
		output:   output,
		input:    input,
	}
	aliveCells := readImage(params, controllerChannels)

	//Connect to RPC Server

	//Dial broker address.
	client, _ := rpc.Dial("tcp", *brokerAddr)
	status := new(stubs.StatusReport)
	//Create a new buffered channel
	//client.Call(stubs.CreateChannel, stubs.ChannelRequest{Topic: topic, Buffer: 10}, status)
	publishParams := stubs.PublishParams{
		Alive:  aliveCells,
		Params: params,
	}
	events := make(chan gol.Event)
	towork := stubs.PublishRequest{Topic: topic, Params: &publishParams, Events: events}
	//Call the broker
	//go handleChannels(client, events)
	err := client.Call(stubs.Publish, towork, status)

	if err != nil {
		fmt.Println("RPC client returned error:")
		fmt.Println(err)
		fmt.Println("Shutting down miner.")
	} else {
		fmt.Println("Completed turns")
		fmt.Println("Alive:", status.Alive)
		fmt.Println("Turns:", status.Turns)
	}
}

func readImage(p gol.Params, c controllerChannels) []util.Cell {

	aliveCells := []util.Cell{}

	c.command <- ioCheckIdle
	//fmt.Println("Sent idle check")
	for {
		idle := false
		select {
		case x := <-c.ioIdle:
			idle = x
		}
		if idle {
			break
		}
	}

	c.command <- ioInput

	s := fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)
	c.filename <- s

	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			select {
			case b := <-c.input:
				if b == 255 {
					aliveCells = append(aliveCells, util.Cell{X: j, Y: i})
				}
			}
		}
	}

	return aliveCells
}

func handleChannels(client *rpc.Client, events chan gol.Event) {
	t := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-t.C:
			aliveReport := stubs.AliveReport{Alive: 0, Turn: 0}
			client.Call(stubs.ReturnAlive, nil, &aliveReport)
			fmt.Println("Completed turn:", aliveReport.Turn, "   Alive Cells:", aliveReport.Alive)
		case <-events:
			fmt.Println("Received event")
		}
	}
}
