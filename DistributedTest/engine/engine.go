package main

import (
	"flag"
	"net"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/stubs"
)

type Engine struct{}

func handleInput(input chan stubs.PublishParams) {
	for {
		job := <-input
		gol.Run(job.Params, nil, nil, job.Alive)
	}
}

// main is the function called when starting Game of Life with 'go run .'
func main() {
	/*runtime.LockOSThread()
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

	flag.Parse()

	fmt.Println("Threads:", params.Threads)
	fmt.Println("Width:", params.ImageWidth)
	fmt.Println("Height:", params.ImageHeight)

	keyPresses := make(chan rune, 10)
	events := make(chan gol.Event, 1000)

	gol.Run(params, events, keyPresses)
	sdl.Start(params, events, keyPresses)*/

	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&Engine{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
