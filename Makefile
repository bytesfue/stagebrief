.PHONY: run build release

VERSION ?= dev

run:
	go run ./cmd/notify

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/notify ./cmd/notify

release:
	@if [ -z "$(TAG)" ]; then echo "Usage: make release TAG=v0.1.4"; exit 1; fi
	@case "$(TAG)" in \
		v*.*.*) ;; \
		*) echo "Error: TAG '$(TAG)' doesn't match the release workflow's trigger pattern v*.*.* — no image would be built. Use e.g. TAG=v0.2.0"; exit 1 ;; \
	esac
	git checkout develop
	git pull
	git checkout main
	git pull
	git merge --ff-only develop
	git push
	git tag $(TAG)
	git push origin $(TAG)
	git checkout develop
	@echo "✓ Released $(TAG) — check GitHub Actions for build status"