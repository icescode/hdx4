package main

import "github.com/faiface/beep/speaker"

func cmdPlayVolume(idx int, loop bool) {
	stateMu.Lock()
	defer stateMu.Unlock()

	state.VolumeIndex = idx
	state.TrackIndex = 0
	state.Loop = loop
	state.Playing = true
	state.Paused = false
}

func cmdPause() {
	stateMu.Lock()
	state.Paused = true
	stateMu.Unlock()
}

func cmdResume() {
	stateMu.Lock()
	state.Paused = false
	stateMu.Unlock()
}

func cmdSetVol(db float64) {
	stateMu.Lock()
	state.VolumeDB = db
	stateMu.Unlock()
}
func cmdNext() {
	stateMu.Lock()
	state.Skip = true
	stateMu.Unlock()
}

func cmdStop() {
	stateMu.Lock()
	state.Playing = false
	state.Paused = false
	state.TrackIndex = 0
	stateMu.Unlock()
	speaker.Clear()
}
