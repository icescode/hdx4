.PHONY: all build clean install

# Lokasi folder build absolut
BUILD_DIR = $(shell pwd)/build

all: clean build install

clean:
	clear
	@echo "Cleaning binaries..."
	rm -rf $(BUILD_DIR)

build:
	clear
	@echo "--- building HDX-FLAC2WAV---"
	go build -o hdx-flac2wav cmd/hdx-flac2wav/main.go

	@echo "--- building HDX-STRUCT---"
	go build -o hdx-struct cmd/hdx-struct/main.go

	@echo "--- building HDX-VOLMAKER---"
	go build -o hdx-volmaker cmd/hdx-volmaker/*.go

	@echo "--- building HDX-META---"
	go build -o hdx-meta cmd/hdx-meta/main.go

	@echo "--- building HDX-PLAYER-DEBUG ---"
	go build -o hdx-player-debug cmd/hdx-player-debug/main.go

	@echo "--- building HDX-SERVER ---"
	go build -o hdx-server cmd/hdx-server/*.go

	@echo "--- building HDX-CLIENT ---"
	go build -o hdx-client cmd/hdx-client/main.go
	@echo "--- BUILD END ---"

install:
	clear
	@echo "Moving binaries to ./build"
	mkdir -p $(BUILD_DIR)
	
	mv hdx-flac2wav $(BUILD_DIR)/
	mv hdx-struct $(BUILD_DIR)/
	mv hdx-volmaker $(BUILD_DIR)/
	mv hdx-meta $(BUILD_DIR)/
	mv hdx-player-debug $(BUILD_DIR)/
	mv hdx-server $(BUILD_DIR)/
	mv hdx-client $(BUILD_DIR)/
	@echo "Done. All files are in $(BUILD_DIR)"