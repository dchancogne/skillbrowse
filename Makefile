.PHONY: build test test-race lint vuln

build:
	go build -o bin/skillbrowse ./cmd/skillbrowse

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	golangci-lint run ./...

vuln:
	govulncheck ./...
