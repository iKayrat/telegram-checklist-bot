BINARY      := checklist-bot
CONFIG      := config.json
MIGRATIONS  := migrations

PI_USER := raspberry
PI_HOST := 192.168.0.10
PI_DIR  := /home/raspberry/telegram-checklist-bot
PI      := $(PI_USER)@$(PI_HOST)

.PHONY: run build build-arm64 build-arm test vet tidy clean deploy

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

## Full update: git pull, rebuild for Pi, ship binary+config+migrations,
## restart systemd. Overwrites the Pi's config.json with the local one —
## keep config.json edits on the Mac, not directly on the Pi.
deploy:
	git pull
	$(MAKE) build-arm64
	scp $(BINARY)-arm64 $(PI):$(PI_DIR)/$(BINARY)
	scp $(CONFIG) $(PI):$(PI_DIR)/$(CONFIG)
	ssh $(PI) 'chmod +x $(PI_DIR)/$(BINARY) && sudo systemctl restart checklist-bot && sudo systemctl status checklist-bot --no-pager'

snapshot:
	scp raspberry@192.168.0.10:/tmp/bot-snapshot.db ~/GolandProjects/tgbot/data/bot-snapshot.db