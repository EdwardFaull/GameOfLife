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

type Request interface {
}

type BaseReport interface {
}

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
	InboundIP      string
}

/*
type ChannelRequest struct {
	Topic  string
	Buffer int
}
*/
type Subscription struct {
	FactoryAddress string
	Callback       string
}

/*
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
	OutboundIP string
}

type StatusReport struct {
	Alive      []util.Cell
	Turns      int
	OutboundIP string
}

type ReportRequest struct {
	InboundIP string
}

type KeyPressRequest struct {
	Key       rune
	InboundIP string
}

type KeyPressReport struct {
	Alive      []util.Cell
	Turns      int
	State      State
	OutboundIP string
}
