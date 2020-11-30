package stubs

import (
	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/util"
)

var CreateChannel = "Engine.CreateChannel"
var Publish = "Engine.Publish"
var Subscribe = "Engine.Subscribe"
var ReturnAlive = "Engine.ReturnAlive"
var Initialise = "Engine.Initialise"
var Report = "Engine.Report"
var Tick = "Engine.Tick"
var KeyPress = "Engine.KeyPress"

type InitParams struct {
	Alive  []util.Cell
	Params gol.Params
}

//Structure used by controller to send initial GoL parameters
//to server. Contains initially alive cells, image dimensions
//and turns to be executed
type InitRequest struct {
	Params     *InitParams
	Events     chan gol.Event
	Keypresses chan rune
}

/*
type ChannelRequest struct {
	Topic  string
	Buffer int
}

type Subscription struct {
	Topic          string
	FactoryAddress string
	Callback       string
}

type JobReport struct {
	Alive []util.Cell
	Turns int
}
*/

type AliveReport struct {
	Alive int
	Turn  int
}

type StatusReport struct {
	Alive []util.Cell
	Turns int
}

type ReportRequest struct {
	//
}

type TickRequest struct {
	//
}

type KeyPressRequest struct {
	Key rune
}
