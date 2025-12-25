package main

import (
	"sync"
)

type PlayerState struct {
	VolumeIndex int
	TrackIndex  int
	Playing     bool
	Paused      bool
	Loop        bool
	VolumeDB    float64

	Skip bool // <<< TAMBAHAN
}

var (
	state   = PlayerState{VolumeDB: 0.0}
	stateMu sync.Mutex
)
