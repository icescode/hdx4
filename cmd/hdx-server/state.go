/*
 * Copyright (c) 2025 Hardiyanto Y -Ebiet.
 * This software is part of the HDX (Hardix Audio) project.
 * This code is provided "as is", without warranty of any kind.
 */
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
	EventSink   func(string)
	Skip        bool // <<< TAMBAHAN
}

var (
	state   = PlayerState{VolumeDB: 0.0}
	stateMu sync.Mutex
)
