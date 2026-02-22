.PHONY: build run clean

BINARY_NAME=vaults3
BUILD_DIR=.

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/vaults3

run: build
	./$(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME)
	rm -rf data/ metadata/

test:
	go test ./...

fmt:
	go fmt ./...
