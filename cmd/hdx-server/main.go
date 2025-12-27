/*
 * Copyright (c) 2025 Hardiyanto Y -Ebiet.
 * This software is part of the HDX (Hardix Audio) project.
 * This code is provided "as is", without warranty of any kind.
 */

package main

const (
	storage_data  = ".hdx-server-data"
	socket_file   = "/tmp/hdx-server.sock"
	version_major = 1
	version_minor = 0
	server_name   = "HDX-Server"
)

func main() {
	loadVolumes()
	go engineLoop()
	startIPC()
}
