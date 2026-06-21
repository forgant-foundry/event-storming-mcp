.PHONY: build test clean run

build:
	go build -o bin/event-storming-mcp .

test:
	go test ./...

clean:
	rm -rf bin/

run: build
	./bin/event-storming-mcp serve
