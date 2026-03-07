.PHONY: start stop start-dev stop-dev test test-integration migrate-up migrate-down seed lint lint-fix

# Load .env.local if present (KEY=value, one per line; comments on own line)
ifneq (,$(wildcard .env.local))
include .env.local
export
endif

start:
	docker compose -f docker-compose.yaml --profile app up --build -d --wait
	sleep 5 # wait for the services to be ready
	
	# Run migrations and seed
	make migrate-down
	make migrate-up
	make seed

stop:
	docker compose -f docker-compose.yaml --profile app down

start-dev:
	docker compose -f docker-compose.yaml --profile dependencies up --build -d --wait
	sleep 5 # wait for the services to be ready
	
	# Run migrations and seed
	make migrate-down
	make migrate-up
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
	go run ./cmd/seed

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix
