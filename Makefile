.PHONY: build test test-integration lint lint-fix

build:
	CGO_ENABLED=0 go build -o bin/monstera-fed ./cmd/monstera-fed

test:
	go test -race -count=1 ./...

test-integration:
	go test -race -count=1 -tags=integration ./...

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix
