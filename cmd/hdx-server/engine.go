/*
 * Copyright (c) 2025 Hardiyanto Y -Ebiet.
 * This software is part of the HDX (Hardix Audio) project.
 * This code is provided "as is", without warranty of any kind.
 */

package main

import (
	"encoding/binary"
	"io"
	"os"
	"time"

	"hardix/internal/security"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
	"github.com/hraban/opus"
)

// ======================================================
// Lazy Opus Streamer (reuse proven logic)
// ======================================================
type lazyStreamer struct {
	file   *os.File
	key    []byte
	dec    *opus.Decoder
	buffer [][2]float64
}

func (l *lazyStreamer) Stream(samples [][2]float64) (int, bool) {
	filled := 0

	for filled < len(samples) {
		if len(l.buffer) == 0 {
			var sz uint16
			if err := binary.Read(l.file, binary.BigEndian, &sz); err != nil {
				return filled, false
			}

			enc := make([]byte, sz)
			if _, err := io.ReadFull(l.file, enc); err != nil {
				return filled, false
			}

			dec, err := security.Decrypt(enc, l.key)
			if err != nil {
				continue
			}

			out := make([]int16, 5760*2)
			n, err := l.dec.Decode(dec, out)
			if err != nil {
				continue
			}

			for i := 0; i < n; i++ {
				l.buffer = append(l.buffer, [2]float64{
					float64(out[i*2]) / 32768.0,
					float64(out[i*2+1]) / 32768.0,
				})
			}
		}

		n := copy(samples[filled:], l.buffer)
		l.buffer = l.buffer[n:]
		filled += n
	}

	return filled, true
}

func (l *lazyStreamer) Err() error { return nil }

// ======================================================
// Runtime audio handles (live control)
// ======================================================
type RuntimeAudio struct {
	Ctrl   *beep.Ctrl
	Volume *effects.Volume
}

var runtime RuntimeAudio

// ======================================================
// Engine Loop (single authority)
// ======================================================
func engineLoop() {
	sr := beep.SampleRate(48000)
	speaker.Init(sr, sr.N(time.Millisecond*100))

	for {
		// --- wait until PLAYING ---
		stateMu.Lock()
		if !state.Playing {
			stateMu.Unlock()
			time.Sleep(40 * time.Millisecond)
			continue
		}

		vIdx := state.VolumeIndex
		tIdx := state.TrackIndex
		loop := state.Loop
		paused := state.Paused
		volDB := state.VolumeDB
		stateMu.Unlock()

		// --- safety ---
		if vIdx < 0 || vIdx >= len(volumes) {
			time.Sleep(40 * time.Millisecond)
			continue
		}

		meta := volumes[vIdx].Meta
		if tIdx < 0 || tIdx >= len(meta.Content) {
			stateMu.Lock()
			if loop {
				state.TrackIndex = 0
			} else {
				state.Playing = false
			}
			stateMu.Unlock()
			continue
		}

		track := meta.Content[tIdx]

		// --- open audio ---
		f, err := os.Open(volumes[vIdx].Path)
		if err != nil {
			time.Sleep(40 * time.Millisecond)
			continue
		}
		f.Seek(int64(track.Offset), io.SeekStart)

		ls := &lazyStreamer{
			file: f,
			key:  volumes[vIdx].AudioKey,
		}
		ls.dec, _ = opus.NewDecoder(48000, 2)

		vol := &effects.Volume{
			Streamer: ls,
			Base:     2,
			Volume:   volDB,
		}
		ctrl := &beep.Ctrl{
			Streamer: vol,
			Paused:   paused,
		}

		// expose runtime handles
		runtime.Volume = vol
		runtime.Ctrl = ctrl

		done := make(chan struct{})
		speaker.Clear()
		speaker.Play(beep.Seq(ctrl, beep.Callback(func() {
			close(done)
		})))

		// ==================================================
		// TRACK LOOP (live control)
		// ==================================================
	TRACK_LOOP:
		for {
			select {
			case <-done:
				break TRACK_LOOP
			default:
				stateMu.Lock()

				// STOP
				if !state.Playing {
					stateMu.Unlock()
					speaker.Clear()
					f.Close()
					goto NEXT_TRACK
				}

				// NEXT (skip current track)
				if state.Skip {
					state.Skip = false
					state.TrackIndex++
					stateMu.Unlock()
					speaker.Clear()
					f.Close()
					goto NEXT_TRACK
				}

				// LIVE UPDATE
				ctrl.Paused = state.Paused
				vol.Volume = state.VolumeDB

				stateMu.Unlock()
				time.Sleep(25 * time.Millisecond)
			}
		}

		f.Close()

		// --- track finished naturally ---
		stateMu.Lock()
		if state.Playing {
			state.TrackIndex++
		}
		stateMu.Unlock()

	NEXT_TRACK:
		continue
	}
}
