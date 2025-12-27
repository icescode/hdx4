/*
 * Copyright (c) 2025 Hardiyanto Y -Ebiet.
 * This software is part of the HDX (Hardix Audio) project.
 * This code is provided "as is", without warranty of any kind.
 */

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
	listPath = filepath.Join(home, storage_data)

	if _, err := os.Stat(listPath); os.IsNotExist(err) {
		_ = os.WriteFile(listPath, []byte(""), 0644)
	}

	_ = os.Remove(socket_file)
	ln, err := net.Listen("unix", socket_file)
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

		// ==================================================
		// PARSE COMMAND: VERB + RAW ARG (AMAN SPASI)
		// ==================================================
		parts := strings.SplitN(line, " ", 2)
		cmd := strings.ToUpper(parts[0])

		// ==================================================
		// READ-ONLY COMMANDS (TIDAK BUTUH OWNER)
		// ==================================================
		switch cmd {

		case "ABOUT":
			aboutStr := fmt.Sprintf(
				"%s V.%d.%d\n",
				server_name,
				version_major,
				version_minor,
			)
			c.Write([]byte(aboutStr))
			continue

		case "PING":
			c.Write([]byte("Pong\n"))
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
			if len(out) == 0 {
				c.Write([]byte("NO VOLUME YET\n"))
			} else {
				j, _ := json.Marshal(out)
				c.Write(append(j, '\n'))
			}
			continue

		case "LIST-HDXV":
			if len(parts) != 2 {
				c.Write([]byte("ERR ARG\n"))
				continue
			}

			idx, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil || idx < 0 || idx >= len(volumes) {
				c.Write([]byte("ERR ARG\n"))
				continue
			}

			var tracks []map[string]interface{}
			for _, t := range volumes[idx].Meta.Content {
				tracks = append(tracks, map[string]interface{}{
					"track_number": t.TrackNumber,
					"title":        t.Title,
					"artist":       volumes[idx].Meta.Artist,
					"duration":     t.Duration,
				})
			}

			j, _ := json.Marshal(tracks)
			c.Write(append(j, '\n'))
			continue
		}

		// ==================================================
		// CONTROL COMMANDS (BUTUH OWNER)
		// ==================================================
		if !claimOwner(c) {
			c.Write([]byte("ERR CONTROL_LOCKED\n"))
			continue
		}

		// Safe EventSink
		stateMu.Lock()
		state.EventSink = func(msg string) {
			if _, err := c.Write([]byte(msg + "\n")); err != nil {
				releaseOwner(c)
			}
		}
		stateMu.Unlock()

		switch cmd {

		case "REMOVE-VOLUME":
			if len(parts) != 2 {
				c.Write([]byte("ERR ARG\n"))
				continue
			}

			idx, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil || idx < 0 || idx >= len(volumes) {
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

			data, err := os.ReadFile(listPath)
			if err != nil {
				c.Write([]byte("ERR INTERNAL\n"))
				continue
			}

			var newLines []string
			for _, l := range strings.Split(string(data), "\n") {
				l = strings.TrimSpace(l)
				if l == "" || l == volumes[idx].Path {
					continue
				}
				newLines = append(newLines, l)
			}

			err = os.WriteFile(
				listPath,
				[]byte(strings.Join(newLines, "\n")+"\n"),
				0644,
			)
			if err != nil {
				c.Write([]byte("ERR INTERNAL\n"))
				continue
			}

			volumes = append(volumes[:idx], volumes[idx+1:]...)
			c.Write([]byte("OK\n"))

		case "ADD-VOLUME":
			if len(parts) != 2 {
				c.Write([]byte("ERR ARG\n"))
				continue
			}

			hdxvPath := strings.TrimSpace(parts[1])
			fmt.Printf("ADD-VOLUME path = %q\n", hdxvPath)

			data, _ := os.ReadFile(listPath)

			for _, l := range strings.Split(string(data), "\n") {
				if strings.TrimSpace(l) == hdxvPath {
					c.Write([]byte("ERR DUPLICATE\n"))
					goto next
				}
			}

			f, err := os.Open(hdxvPath)
			if err != nil {
				c.Write([]byte("ERR HDXV_NOT_FOUND\n"))
				goto next
			}

			magic := make([]byte, len(spec.VolumeMagicV2))
			_, err = io.ReadFull(f, magic)
			f.Close()
			if err != nil || string(magic) != spec.VolumeMagicV2 {
				c.Write([]byte("ERR INVALID_MAGIC\n"))
				goto next
			}

			datPath := strings.TrimSuffix(hdxvPath, ".hdxv") + "_keys.dat"
			if _, err := os.Stat(datPath); err != nil {
				c.Write([]byte("ERR KEY_NOT_FOUND\n"))
				goto next
			}

			if _, err := security.UnlockKeyLocker(hdxvPath, datPath, ""); err != nil {
				c.Write([]byte("ERR AUTH_FAILED\n"))
				goto next
			}

			fc, err := os.OpenFile(listPath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				c.Write([]byte("ERR INTERNAL\n"))
				goto next
			}
			fmt.Fprintln(fc, hdxvPath)
			fc.Close()

			loadVolumes()
			c.Write([]byte("Volume Loaded\n"))

		case "PLAY-VOLUME":
			if len(parts) != 2 {
				c.Write([]byte("ERR ARG\n"))
				continue
			}

			v, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil || v < 0 || v >= len(volumes) {
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
			c.Write([]byte("Volume Playing\n"))

		case "PLAY-TRACK":
			if len(parts) != 2 {
				c.Write([]byte("ERR ARG\n"))
				continue
			}

			args := strings.Fields(parts[1])
			if len(args) != 2 {
				c.Write([]byte("ERR ARG\n"))
				continue
			}

			v, err1 := strconv.Atoi(args[0])
			t, err2 := strconv.Atoi(args[1])
			if err1 != nil || err2 != nil {
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
			c.Write([]byte("Track Playing\n"))

		case "STOP":
			cmdStop()
			c.Write([]byte("Stopped\n"))

		case "NEXT":
			cmdNext()
			c.Write([]byte("Jump\n"))

		case "PAUSE":
			cmdPause()
			c.Write([]byte("Paused\n"))

		case "RESUME":
			cmdResume()
			c.Write([]byte("Resume Playing\n"))

		default:
			c.Write([]byte("ERR UNKNOWN\n"))
		}
	next:
	}
}
