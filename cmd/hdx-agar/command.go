/*
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
*/
package main

import (
	"encoding/json"
)

func emitEvent(t string) {
	if state.EventSink == nil {
		return
	}
	stateMu.Lock()
	ev := map[string]interface{}{
		"type":         t,
		"playing":      state.Playing,
		"paused":       state.Paused,
		"volume_index": state.VolumeIndex,
		"track_index":  state.TrackIndex,
		"volume_db":    state.VolumeDB,
	}
	stateMu.Unlock()

	b, _ := json.Marshal(ev)
	state.EventSink("EVENT " + string(b))
}

func cmdPlayTrack(v, t int) {
	stateMu.Lock()
	state.VolumeIndex = v
	state.TrackIndex = t
	state.Playing = true
	state.Paused = false
	state.Skip = false
	stateMu.Unlock()

	emitEvent("TRACK_CHANGED")
}

func cmdPlayVolume(v int, loop bool) {
	stateMu.Lock()
	state.VolumeIndex = v
	state.TrackIndex = 0
	state.Playing = true
	state.Loop = loop
	state.Paused = false
	state.Skip = false
	stateMu.Unlock()

	emitEvent("STATUS")
}

func cmdStop() {
	stateMu.Lock()
	state.Playing = false
	state.Paused = false
	stateMu.Unlock()

	emitEvent("STOPPED")
}

func cmdPause() {
	stateMu.Lock()
	state.Paused = true
	stateMu.Unlock()

	emitEvent("STATUS")
}

func cmdResume() {
	stateMu.Lock()
	state.Paused = false
	stateMu.Unlock()

	emitEvent("STATUS")
}

func cmdNext() {
	stateMu.Lock()
	state.Skip = true
	stateMu.Unlock()
}
