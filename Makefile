BINARY      := checklist-bot
CONFIG      := config.json
MIGRATIONS  := migrations

.PHONY: run build build-arm64 build-arm test vet tidy clean

## Run the bot locally (uses ./config.json and ./migrations by default).
run:
	go run ./cmd/bot -config $(CONFIG) -migrations $(MIGRATIONS)

## Build a binary for the current OS/arch.
build:
	go build -o $(BINARY) ./cmd/bot

## Cross-compile for 64-bit Raspberry Pi OS (Pi 3/4/5).
build-arm64:
	GOOS=linux GOARCH=arm64 go build -o $(BINARY)-arm64 ./cmd/bot

## Cross-compile for 32-bit Raspberry Pi OS (Pi Zero/1/2).
build-arm:
	GOOS=linux GOARCH=arm GOARM=6 go build -o $(BINARY)-arm ./cmd/bot

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -f $(BINARY) $(BINARY)-arm64 $(BINARY)-arm
