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
	input := make(chan byte)
	output := make(chan byte)
	filename := make(chan string)

	distributorChannels := distributorChannels{
		events,
		ioCommand,
		ioIdle,
		input,
		output,
		filename,
		keyPresses,
		make(chan Event),
		make([]chan rune, p.Threads),
		make([]chan filler, p.Threads),
		make(chan filler),
		make([]chan bool, p.Threads),
	}
	go distributor(p, distributorChannels)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: filename,
		output:   output,
		input:    input,
	}
	go startIo(p, ioChannels)

}
