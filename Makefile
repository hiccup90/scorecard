.PHONY: help dev build test check web api docker clean lint fmt

export ALLOW_DEFAULT_PIN ?= 1
export SCORECARD_DEV ?= 1
export CGO_ENABLED ?= 1

help:
	@echo "Targets: dev build test check web api docker clean lint fmt"

dev: web
	go run ./cmd/scorecard

web:
	npm --prefix web install
	npm --prefix web run build

api:
	go build -o dist/scorecard ./cmd/scorecard

build: web api

test:
	go test ./... -count=1

check: test web

lint:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './legacy/*')

docker:
	docker build -t scorecard:local .

clean:
	rm -rf dist web/dist data/*.db data/*.db-*

run-bin: build
	./dist/scorecard
