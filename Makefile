.PHONY: run build

run:
	go run ./cmd/notify

build:
	go build -o bin/notify ./cmd/notify