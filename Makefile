.PHONY: run build

VERSION ?= dev

run:
	go run ./cmd/notify

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/notify ./cmd/notify