package gol

import "uk.ac.bris.cs/gameoflife/util"

type workerParams struct {
	startX int
	startY int
	endX   int
	endY   int
}

func worker(world [][]byte, wp workerParams) {

}

func calculateNextStateP(p Params, world [][]byte, c distributorChannels, completedTurns int) [][]byte {
	h := p.ImageHeight
	w := p.ImageWidth
	nworld := make([][]byte, h, w)
	for y, a := range world {
		nA := make([]byte, w)
		for x, b := range a {
			nB := byte(0)
			ln := calculateAliveNeighbours(h, w, world, x, y)
			if b == 0 {
				if ln == 3 {
					nB = 255
					sendFlippedEvent(x, y, completedTurns, c)
				} else {
					nB = 0
				}
			} else {
				if ln < 2 || ln > 3 {
					nB = 0
					sendFlippedEvent(x, y, completedTurns, c)
				} else {
					nB = 255
				}
			}
			nA[x] = nB
		}
		nworld[y] = nA
	}
	return nworld
}

func sendFlippedEventP(x int, y int, completedTurns int, c distributorChannels) {
	cell := util.Cell{X: x, Y: y}
	c.events <- CellFlipped{CompletedTurns: completedTurns, Cell: cell}
}

func calculateAliveCellsP(p Params, world [][]byte) []util.Cell {
	alive := []util.Cell{}
	for y, a := range world {
		for x, b := range a {
			if b == 255 {
				coord := util.Cell{X: x, Y: y}
				alive = append(alive, coord)
			}
		}
	}
	return alive
}

func calculateAliveNeighboursP(h int, w int, world [][]byte, x int, y int) int {
	ans := 0
	up := (x + 1) % h
	down := (x - 1) % h
	if down < 0 {
		down = down + h
	}
	right := (y + 1) % w
	left := (y - 1) % w
	if left < 0 {
		left = left + w
	}

	if world[left][x] == 255 {
		ans++
	}
	if world[right][x] == 255 {
		ans++
	}
	if world[y][down] == 255 {
		ans++
	}
	if world[y][up] == 255 {
		ans++
	}
	if world[right][down] == 255 {
		ans++
	}
	if world[right][up] == 255 {
		ans++
	}
	if world[left][up] == 255 {
		ans++
	}
	if world[left][down] == 255 {
		ans++
	}
	return ans
}
