.PHONY: build test lint clean release-snapshot

build:
	CGO_ENABLED=0 go build -o bin/gitfive ./cmd/gitfive

test:
	go test -race ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

release-snapshot:
	goreleaser release --snapshot --clean
