.PHONY: build test clean run-events run-stories install

build:
	go build -o bin/forgant-discovery ./cmd/forgant-discovery

test:
	go test ./...

install:
	go install ./cmd/forgant-discovery

clean:
	rm -rf bin/

run-events: build
	./bin/forgant-discovery serve events

run-stories: build
	./bin/forgant-discovery serve stories
