package main

import (
	"flag"
	"runtime"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/sdl"
)

func main() {
	runtime.LockOSThread()
	var params gol.ClientParams

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

	flag.IntVar(
		&params.ShouldContinue,
		"c",
		0,
		"Specify if the controller should resume the previous game of life. 1 to continue, 0 to create a new game. Defaults to 0")

	brokerAddr := flag.String(
		"broker",
		"127.0.0.1:8030",
		"Address of broker instance")

	flag.Parse()

	params.BrokerAddr = *brokerAddr

	events := make(chan gol.Event, 1000)
	keyPresses := make(chan rune, 10)

	gol.Run(params, events, keyPresses)
	sdl.Start(gol.ClientToEngineParams(params), events, keyPresses)
}
