.PHONY: start stop start-dev stop-dev test test-integration migrate-up migrate-down seed lint lint-fix loadtest

# Loadtest configuration (override on the command line: make loadtest LOADTEST_INBOX_REQUESTS=5000)
LOADTEST_USERNAME          ?= alice
LOADTEST_INBOX_REQUESTS    ?= 10000
LOADTEST_INBOX_CONCURRENCY ?= 20
LOADTEST_FANOUT_FOLLOWERS  ?= 5000
LOADTEST_FANOUT_INSTANCES  ?= 5000

# Load .env.local if present (KEY=value, one per line; comments on own line)
ifneq (,$(wildcard .env.local))
include .env.local
export
endif

start:
	docker compose -f docker-compose.yaml --profile app up --build -d --wait
	sleep 5 # wait for the services to be ready
	
	# Reset and seed database
	make migrate-down
	make seed

stop:
	docker compose -f docker-compose.yaml --profile app down

start-dev:
	docker compose -f docker-compose.yaml --profile dependencies up --build -d --wait
	sleep 5 # wait for the services to be ready
	
	# Reset and seed database
	make migrate-down
	make seed

	# Run server and UI in parallel
	@trap 'kill $$MONSTERA_PID $$UI_PID 2>/dev/null; docker compose -f docker-compose.yaml --profile dependencies down; exit 0' INT TERM; \
	go run ./cmd/server serve & MONSTERA_PID=$$!; \
	(cd ui && npm run dev) & UI_PID=$$!; \
	wait

test:
	go test -race -count=1 ./...

test-integration:
	# Start the dependencies
	docker compose -f docker-compose.yaml --profile dependencies up -d --wait
	sleep 5 # wait for the services to be ready

	# Run migrations
	make migrate-down
	make migrate-up

	# Run the tests
	go test -race -count=1 -tags=integration ./...; \
	EXIT=$$?; \
	docker compose -f docker-compose.yaml --profile dependencies down; \
	exit $$EXIT

migrate-up:
	go run ./cmd/server migrate up

migrate-down:
	go run ./cmd/server migrate down-all || true

seed:
	go run ./cmd/server user create admin admin@example.com password Admin admin
	go run ./cmd/server user create moderator mod@example.com password Moderator moderator
	go run ./cmd/server user create alice alice@example.com password Alice user

loadtest:
	# 1. Start full stack, reset database, and seed
	docker compose -f docker-compose.yaml --profile app up --build -d --wait
	sleep 5
	$(MAKE) migrate-down
	$(MAKE) seed

	# 2. Build static Linux binary and deploy into the server container
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/loadtest-linux ./cmd/loadtest/
	docker cp /tmp/loadtest-linux monstera-server-1:/tmp/loadtest

	# 3. Provision token, run tests, always tear down
	# TODO: move loadtest into a docker compose service for portability and to centralize config
	@set -e; \
	trap 'docker compose -f docker-compose.yaml --profile app down' EXIT; \
	TOKEN=$$(docker exec monstera-server-1 /tmp/loadtest setup \
	  --db-url "postgres://monstera:monstera@postgres:5432/monstera?sslmode=disable" \
	  --username $(LOADTEST_USERNAME)); \
	echo "=== Inbox flood ==="; \
	docker exec monstera-server-1 /tmp/loadtest inbox \
	  --target http://localhost:8080 \
	  --username $(LOADTEST_USERNAME) \
	  --total $(LOADTEST_INBOX_REQUESTS) \
	  --concurrency $(LOADTEST_INBOX_CONCURRENCY); \
	echo "=== Fanout ==="; \
	docker exec monstera-server-1 /tmp/loadtest fanout \
	  --username $(LOADTEST_USERNAME) \
	  --followers $(LOADTEST_FANOUT_FOLLOWERS) \
	  --instances $(LOADTEST_FANOUT_INSTANCES) \
	  --server-url http://localhost:8080 \
	  --db-url "postgres://monstera:monstera@postgres:5432/monstera?sslmode=disable" \
	  --nats-url "nats://nats:4222" \
	  --token "$$TOKEN" \
	  --timeout 60s \
	  --cleanup

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix
