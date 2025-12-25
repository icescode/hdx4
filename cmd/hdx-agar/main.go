package main

func main() {
	loadVolumes()
	go engineLoop()
	startIPC()
}
