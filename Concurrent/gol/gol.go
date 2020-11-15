package gol

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {

	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: nil,
		output:   nil,
		input:    make(chan<- uint8),
	}
	go startIo(p, ioChannels)

	distributorChannels := distributorChannels{
		events,
		ioCommand,
		ioIdle,
		ioChannels
	}
	go distributor(p, distributorChannels)
}
