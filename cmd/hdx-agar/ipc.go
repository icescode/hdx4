package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func startIPC() {
	os.Remove("/tmp/hdx-agar.sock")
	ln, _ := net.Listen("unix", "/tmp/hdx-agar.sock")

	for {
		c, _ := ln.Accept()
		go handleConn(c)
	}
}

func handleConn(c net.Conn) {
	defer c.Close()
	sc := bufio.NewScanner(c)

	for sc.Scan() {
		cmd := strings.TrimSpace(sc.Text())

		switch {
		case strings.HasPrefix(cmd, "PLAY_VOLUME_LOOP"):
			var i int
			fmt.Sscanf(cmd, "PLAY_VOLUME_LOOP %d", &i)
			cmdPlayVolume(i, true)
			c.Write([]byte("OK\n"))

		case strings.HasPrefix(cmd, "PLAY_VOLUME"):
			var i int
			fmt.Sscanf(cmd, "PLAY_VOLUME %d", &i)
			cmdPlayVolume(i, false)
			c.Write([]byte("OK\n"))

		case cmd == "STOP":
			cmdStop()
			c.Write([]byte("OK\n"))

		case cmd == "PAUSE":
			cmdPause()
			c.Write([]byte("OK\n"))

		case cmd == "RESUME":
			cmdResume()
			c.Write([]byte("OK\n"))

		case cmd == "NEXT":
			cmdNext()
			c.Write([]byte("OK\n"))

		case strings.HasPrefix(cmd, "SET_VOL"):
			var v float64
			fmt.Sscanf(cmd, "SET_VOL %f", &v)
			cmdSetVol(v)
			c.Write([]byte("OK\n"))

		default:
			c.Write([]byte("ERR\n"))
		}
	}
}
