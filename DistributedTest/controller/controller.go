package main

import (
	"flag"
	"fmt"
	"net/rpc"
	"runtime"

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

	/*flag.IntVar(
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
		"Specify the number of turns to process. Defaults to 10000000000.")*/

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
	client.Call(stubs.CreateChannel, stubs.ChannelRequest{Topic: topic, Buffer: 10}, status)
	publishParams := stubs.PublishParams{
		Alive:  aliveCells,
		Params: params,
	}
	towork := stubs.PublishRequest{Topic: topic, Params: publishParams}
	//Call the broker
	err := client.Call(stubs.Publish, towork, status)
	if err != nil {
		fmt.Println("RPC client returned error:")
		fmt.Println(err)
		fmt.Println("Shutting down miner.")
	}
}

func readImage(p gol.Params, c controllerChannels) []util.Cell {
	//Create a 2D slice to store the world.
	/*world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}*/

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
				//world[i][j] = b
				if b == 255 {
					aliveCells = append(aliveCells, util.Cell{X: j, Y: i})
				}
			}
		}
	}

	return aliveCells
}
