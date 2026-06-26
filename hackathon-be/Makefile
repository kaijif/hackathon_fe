# NightWatch backend — common tasks.
# On Windows without `make`, run the underlying `go`/`docker` commands directly.

.PHONY: build run test vet fmt tidy db-up db-down up down

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

# Start only Postgres (for local `make run`).
db-up:
	docker compose up -d db

db-down:
	docker compose down

# Start the full stack (Postgres + API) in containers.
up:
	docker compose up --build

down:
	docker compose down
