package gol

import (
	"uk.ac.bris.cs/gameoflife/util"
)

var Subscribe = "Engine.Subscribe"
var Initialise = "Engine.Initialise"
var Report = "Engine.Report"
var Tick = "Engine.Tick"
var KeyPress = "Engine.KeyPress"
var Kill = "Factory.Kill"
var Fetch = "Factory.Fetch"

type Request interface {
}

type BaseReport interface {
}

//Structure used by controller to send initial GoL parameters
//to server. Contains initially alive cells, image dimensions
//and turns to be executed
type InitRequest struct {
	Alive          []util.Cell
	Params         Params
	ShouldContinue int
	InboundIP      string
	Factories      int
	UpperIP        string
	LowerIP        string
	StartY         int
}

type Subscription struct {
	FactoryAddress string
}

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

type KillRequest struct {
}

type KeyPressReport struct {
	Alive      []util.Cell
	Turns      int
	State      State
	OutboundIP string
}

//True if upper
//False if lower
type FetchRequest struct {
	UpperOrLower bool
}

type FetchReport struct {
	Line []byte
}

type SyncRequest struct {
	Turn      int
	InboundIP string
}

type SyncReport struct {
}
