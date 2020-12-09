package gol

import (
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
	Params Params
}

//Structure used by controller to send initial GoL parameters
//to server. Contains initially alive cells, image dimensions
//and turns to be executed
type InitRequest struct {
	Params         *InitParams
	ShouldContinue int
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

type TickReport struct {
	Turns      int
	Alive      []util.Cell
	CellsCount int
	ReportType ReportType
}

type StatusReport struct {
	Alive []util.Cell
	Turns int
}

type ReportRequest struct {
	//
}

type KeyPressRequest struct {
	Key rune
}

type KeyPressReport struct {
	Alive []util.Cell
	Turns int
	State State
}
