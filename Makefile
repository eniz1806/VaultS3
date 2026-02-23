.PHONY: build build-go run clean test fmt web dev-web dev-api

BINARY_NAME=vaults3
BUILD_DIR=.

# Build React frontend
web:
	cd web && npm install && npm run build
	rm -rf internal/dashboard/dist
	cp -r web/dist internal/dashboard/dist

# Build Go binary (includes frontend)
build: web
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/vaults3

# Build Go only (skip frontend, for backend-only dev)
build-go:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/vaults3

run: build
	./$(BINARY_NAME)

# Dev mode: Vite dev server with proxy to Go backend
dev-web:
	cd web && npm run dev

# Dev mode: Go backend only
dev-api: build-go
	./$(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME)
	rm -rf data/ metadata/
	rm -rf internal/dashboard/dist
	rm -rf web/node_modules web/dist

test:
	go test ./...

fmt:
	go fmt ./...
