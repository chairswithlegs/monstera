.PHONY: build build-seed seed test test-integration lint lint-fix

build:
	CGO_ENABLED=0 go build -o bin/monstera-fed ./cmd/monstera-fed

build-seed:
	CGO_ENABLED=0 go build -o bin/seed ./cmd/seed

# seed runs the seeder against the docker-compose stack (postgres on localhost:5433).
# Start the stack with docker compose up -d first.
seed: build-seed
	DATABASE_URL="postgres://monstera:monstera@localhost:5433/monstera_fed?sslmode=disable" \
	NATS_URL="nats://localhost:4222" \
	INSTANCE_DOMAIN=localhost:8080 \
	INSTANCE_NAME="Monstera-fed (local)" \
	MEDIA_BASE_URL=http://localhost:8080/media \
	MEDIA_LOCAL_PATH=./data/media \
	EMAIL_FROM=noreply@localhost \
	SECRET_KEY_BASE="0000000000000000000000000000000000000000000000000000000000000000" \
	./bin/seed

test:
	go test -race -count=1 ./...

# Reset DB and run migrations so integration tests see a fresh schema. Requires docker compose up.
test-integration: build
	DATABASE_URL="postgres://monstera:monstera@localhost:5433/monstera_fed?sslmode=disable" \
		./bin/monstera-fed migrate down-all || true
	DATABASE_URL="postgres://monstera:monstera@localhost:5433/monstera_fed?sslmode=disable" \
		./bin/monstera-fed migrate up
	DATABASE_URL="postgres://monstera:monstera@localhost:5433/monstera_fed?sslmode=disable" \
	NATS_URL="nats://localhost:4222" \
	AWS_ACCESS_KEY_ID=minioadmin AWS_SECRET_ACCESS_KEY=minioadmin \
	MINIO_ENDPOINT=http://localhost:9000 S3_TEST_BUCKET=test-bucket \
	go test -race -count=1 -tags=integration ./...

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix
