.PHONY: build test test-integration lint lint-fix

build:
	CGO_ENABLED=0 go build -o bin/monstera-fed ./cmd/monstera-fed

test:
	go test -race -count=1 ./...

test-integration:
	DATABASE_URL="postgres://monstera:monstera@localhost:5433/monstera_fed?sslmode=disable" \
	NATS_URL="nats://localhost:4222" \
	AWS_ACCESS_KEY_ID=minioadmin AWS_SECRET_ACCESS_KEY=minioadmin \
	MINIO_ENDPOINT=http://localhost:9000 S3_TEST_BUCKET=test-bucket \
	go test -race -count=1 -tags=integration ./...

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix
