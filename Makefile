.PHONY: build run dry-run get-chat-id tidy clean

BIN=bin/digest

build:
	go build -o $(BIN) ./cmd/digest

run: build
	./$(BIN) --once

dry-run: build
	./$(BIN) --once --dry-run

get-chat-id: build
	./$(BIN) --get-chat-id

tidy:
	go mod tidy

clean:
	rm -rf bin
