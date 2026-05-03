APP := easycat
CMD := ./cmd/easycat
BIN_DIR := bin
BIN := $(BIN_DIR)/$(APP)

export GOCACHE ?= /tmp/easycat-go-build
export GOMODCACHE ?= /tmp/easycat-go-mod

.PHONY: deps build dev test clean

deps:
	go mod tidy
	go mod download

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN) $(CMD)

dev:
	cargo watch -s 'go run $(CMD)'

test:
	go test ./...

clean:
	rm -rf $(BIN_DIR)
