package stubs

import "uk/gameoflife/util"

var CreateChannel = "Broker.CreateChannel"
var Publish = "Broker.Publish"
var Subscribe = "Broker.Subscribe"

type parameters struct {
	initialCells []util.Cell
	turns        int
	imageHeight  int
	imageWidth   int
}

type PublishRequest struct {
	Topic string
	p     parameters
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
	Result []util.Cell
}

type StatusReport struct {
	Message string
}
