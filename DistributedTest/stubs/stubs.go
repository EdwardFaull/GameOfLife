package stubs

import (
	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/util"
)

var CreateChannel = "Engine.CreateChannel"
var Publish = "Engine.Publish"
var Subscribe = "Engine.Subscribe"

type PublishParams struct {
	Alive  []util.Cell
	Params gol.Params
}

//Structure used by controller to send initial GoL parameters
//to server. Contains initially alive cells, image dimensions
//and turns to be executed
type PublishRequest struct {
	Topic  string
	Params PublishParams
}

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

type StatusReport struct {
	Message string
}
