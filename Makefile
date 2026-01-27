.PHONY: build test clean run

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/sentinel ./cmd/sentinel

test:
	go test -v -cover ./...

clean:
	rm -rf bin/

run:
	go run ./cmd/sentinel

docker:
	docker build -t sentinel:$(VERSION) .
