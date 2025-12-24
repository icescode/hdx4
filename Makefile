.PHONY: all build clean install

# Lokasi folder build absolut
BUILD_DIR = $(shell pwd)/build

all: clean build install

build:
	@echo "--- COMPILING HARDIX-HDX4 JSFD SUITE ---"
	# Build langsung ke root dulu agar gampang dideteksi jika gagal
	go build -o hdx-jukebox cmd/hdx-jukebox/main.go
	go build -o hdx-meta cmd/hdx-meta/main.go
	go build -o hdx-struct cmd/hdx-struct/main.go	
	go build -o hdx-volmaker cmd/hdx-volmaker/*.go	
	@echo "--- BUILD SUCCESS ---"

clean:
	@echo "Cleaning binaries..."
	rm -rf $(BUILD_DIR)/*
	rm -f hdx-jukebox hdx-meta hdx-struct hdx-volmaker

install:
	@echo "Moving binaries to ./build"
	mkdir -p $(BUILD_DIR)
	mv hdx-jukebox hdx-meta hdx-struct hdx-volmaker $(BUILD_DIR)/
	@echo "Done. All files are in $(BUILD_DIR)"