package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

const (
	socket_file        = "/tmp/hdx-server.sock"
	version_major      = 1
	version_minor      = 0
	app_name           = "HDX-Client"
	developer_title    = "Developer Hardiyanto"
	developer_subtitle = "Build 27/12/2025 Ebiet Version"
)

func main() {
	fmt.Printf("\n%s V.%d.%d\n", app_name, version_major, version_minor)
	fmt.Printf("%s %s\n", developer_title, developer_subtitle)
	conn, err := net.Dial("unix", socket_file)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	fmt.Println("CONNECTED (like socat)")
	fmt.Println("Type IPC command, press Enter")
	fmt.Println(`Type "QUIT" to exit`)
	fmt.Println()

	// ============================
	// STDIN → IPC (interactive)
	// ============================
	go func() {
		in := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("hdx> ")
			if !in.Scan() {
				// EOF / stdin closed
				os.Exit(0)
			}

			line := strings.TrimSpace(in.Text())
			if line == "" {
				// prompt tetap muncul
				continue
			}

			// FORCE UPPERCASE
			//line = strings.ToUpper(line)

			if line == "QUIT" {
				fmt.Println("Bye.")
				os.Exit(0)
			}

			_, err := conn.Write([]byte(line + "\n"))
			if err != nil {
				fmt.Println("WRITE ERROR:", err)
				os.Exit(1)
			}
		}
	}()

	// ============================
	// IPC → STDOUT (blocking)
	// ============================
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		fmt.Println("RECV:", sc.Text())
	}

	fmt.Println("SOCKET CLOSED")
	os.Exit(0)
}
