package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"hardix/internal/security"
	"hardix/pkg/spec"
)

// ===============================
// Globals
// ===============================

var (
	controlOwner net.Conn
	controlMu    sync.Mutex
	listPath     string
)

// ===============================
// Ownership
// ===============================

func isOwner(c net.Conn) bool {
	controlMu.Lock()
	defer controlMu.Unlock()
	return controlOwner == c
}

func claimOwner(c net.Conn) bool {
	controlMu.Lock()
	defer controlMu.Unlock()
	if controlOwner == nil {
		controlOwner = c
		return true
	}
	return controlOwner == c
}

func releaseOwner(c net.Conn) {
	controlMu.Lock()
	defer controlMu.Unlock()
	if controlOwner == c {
		controlOwner = nil
		stateMu.Lock()
		state.EventSink = nil
		stateMu.Unlock()
		cmdStop()
	}
}

// ===============================
// IPC Server
// ===============================

func startIPC() {
	home, _ := os.UserHomeDir()
	listPath = filepath.Join(home, ".hdx-socket_list")

	if _, err := os.Stat(listPath); os.IsNotExist(err) {
		_ = os.WriteFile(listPath, []byte(""), 0644)
	}

	_ = os.Remove("/tmp/hdx-agar.sock")
	ln, err := net.Listen("unix", "/tmp/hdx-agar.sock")
	if err != nil {
		panic(err)
	}

	for {
		c, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleConn(c)
	}
}
func argInt(parts []string, idx int) (int, bool) {
	if len(parts) <= idx {
		return 0, false
	}
	v, err := strconv.Atoi(parts[idx])
	if err != nil {
		return 0, false
	}
	return v, true
}
func argFloat(parts []string, idx int) (float64, bool) {
	if len(parts) <= idx {
		return 0, false
	}
	v, err := strconv.ParseFloat(parts[idx], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
func handleConn(c net.Conn) {
	defer func() {
		releaseOwner(c)
		c.Close()
	}()

	sc := bufio.NewScanner(c)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := strings.ToUpper(parts[0])

		// ===============================
		// READ-ONLY
		// ===============================
		switch cmd {

		case "PING":
			c.Write([]byte("OK\n"))
			continue

		case "WHOAMI":
			if isOwner(c) {
				c.Write([]byte("OWNER\n"))
			} else {
				c.Write([]byte("OBSERVER\n"))
			}
			continue

		case "STATUS":
			stateMu.Lock()
			resp := map[string]interface{}{
				"playing":      state.Playing,
				"paused":       state.Paused,
				"loop":         state.Loop,
				"volume_index": state.VolumeIndex,
				"track_index":  state.TrackIndex,
				"volume_db":    state.VolumeDB,
			}
			stateMu.Unlock()
			j, _ := json.Marshal(resp)
			c.Write(append(j, '\n'))
			continue

		case "LIST-VOLUME":
			var out []map[string]interface{}
			for i, v := range volumes {
				out = append(out, map[string]interface{}{
					"index":        i,
					"album":        v.Meta.Album,
					"artist":       v.Meta.Artist,
					"total_tracks": len(v.Meta.Content),
				})
			}
			j, _ := json.Marshal(out)
			c.Write(append(j, '\n'))
			continue

		case "LIST-HDXV":
			idx, ok := argInt(parts, 1)
			if !ok || idx < 0 || idx >= len(volumes) {
				c.Write([]byte("ERR ARG\n"))
				continue
			}
			var tracks []map[string]interface{}
			for _, t := range volumes[idx].Meta.Content {
				tracks = append(tracks, map[string]interface{}{
					"track_number": t.TrackNumber,
					"title":        t.Title,
					"duration":     t.Duration,
				})
			}
			j, _ := json.Marshal(tracks)
			c.Write(append(j, '\n'))
			continue
		}

		// ===============================
		// OWNER ONLY
		// ===============================
		if !claimOwner(c) {
			c.Write([]byte("ERR CONTROL_LOCKED\n"))
			continue
		}

		// Safe EventSink
		stateMu.Lock()
		state.EventSink = func(msg string) {
			_, err := c.Write([]byte(msg + "\n"))
			if err != nil {
				releaseOwner(c)
			}
		}
		stateMu.Unlock()

		switch cmd {

		case "REMOVE-VOLUME":
			idx, ok := argInt(parts, 1)
			if !ok || idx < 0 || idx >= len(volumes) {
				c.Write([]byte("ERR ARG\n"))
				continue
			}

			stateMu.Lock()
			isPlaying := state.Playing
			current := state.VolumeIndex
			stateMu.Unlock()

			if isPlaying && current == idx {
				c.Write([]byte("ERR VOLUME_IN_USE\n"))
				continue
			}

			// remove dari file .hdx-socket_list
			data, err := os.ReadFile(listPath)
			if err != nil {
				c.Write([]byte("ERR INTERNAL\n"))
				continue
			}

			var newLines []string
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if line == volumes[idx].Path {
					continue
				}
				newLines = append(newLines, line)
			}

			if err := os.WriteFile(
				listPath,
				[]byte(strings.Join(newLines, "\n")+"\n"),
				0644,
			); err != nil {
				c.Write([]byte("ERR INTERNAL\n"))
				continue
			}

			// remove dari memory
			volumes = append(volumes[:idx], volumes[idx+1:]...)

			c.Write([]byte("OK\n"))

		case "ADD-VOLUME":
			if len(parts) < 2 {
				c.Write([]byte("ERR ARG\n"))
				continue
			}

			hdxvPath := strings.TrimSpace(parts[1])

			// duplikasi
			// 1. duplikasi
			data, _ := os.ReadFile(listPath)

			duplicate := false
			for _, line := range strings.Split(string(data), "\n") {
				if strings.TrimSpace(line) == hdxvPath {
					duplicate = true
					break
				}
			}
			if duplicate {
				c.Write([]byte("ERR DUPLICATE\n"))
				continue
			}

			// magic header
			f, err := os.Open(hdxvPath)
			if err != nil {
				c.Write([]byte("ERR HDXV_NOT_FOUND\n"))
				continue
			}
			magic := make([]byte, len(spec.VolumeMagicV2))
			_, err = io.ReadFull(f, magic)
			f.Close()
			if err != nil || string(magic) != spec.VolumeMagicV2 {
				c.Write([]byte("ERR INVALID_MAGIC\n"))
				continue
			}

			// key pairing
			datPath := strings.TrimSuffix(hdxvPath, ".hdxv") + "_keys.dat"
			if _, err := os.Stat(datPath); err != nil {
				c.Write([]byte("ERR KEY_NOT_FOUND\n"))
				continue
			}
			if _, err := security.UnlockKeyLocker(hdxvPath, datPath, ""); err != nil {
				c.Write([]byte("ERR AUTH_FAILED\n"))
				continue
			}

			// persist
			fc, err := os.OpenFile(listPath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				c.Write([]byte("ERR INTERNAL\n"))
				continue
			}
			fmt.Fprintln(fc, hdxvPath)
			fc.Close()

			loadVolumes()
			c.Write([]byte("OK\n"))

		case "PLAY-VOLUME":
			v, ok := argInt(parts, 1)
			if !ok || v < 0 || v >= len(volumes) {
				c.Write([]byte("ERR ARG\n"))
				continue
			}

			stateMu.Lock()
			isPlaying := state.Playing
			stateMu.Unlock()

			if isPlaying {
				cmdStop()
			}

			cmdPlayVolume(v, false)
			c.Write([]byte("OK\n"))
		case "PLAY-TRACK":
			v, ok1 := argInt(parts, 1)
			t, ok2 := argInt(parts, 2)
			if !ok1 || !ok2 {
				c.Write([]byte("ERR ARG\n"))
				continue
			}

			if v < 0 || v >= len(volumes) {
				c.Write([]byte("ERR VOLUME_RANGE\n"))
				continue
			}
			if t < 0 || t >= len(volumes[v].Meta.Content) {
				c.Write([]byte("ERR TRACK_RANGE\n"))
				continue
			}

			cmdPlayTrack(v, t)
			c.Write([]byte("OK\n"))

		case "STOP":
			cmdStop()
			c.Write([]byte("OK\n"))

		case "NEXT":
			cmdNext()
			c.Write([]byte("OK\n"))

		case "PAUSE":
			cmdPause()
			c.Write([]byte("OK\n"))

		case "RESUME":
			cmdResume()
			c.Write([]byte("OK\n"))
		default:
			c.Write([]byte("ERR UNKNOWN\n"))
		}
	}
}
