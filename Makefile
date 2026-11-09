.PHONY: dev server build web-build embed test docker clean

VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
MODULE  := $(shell head -1 go.mod | awk '{print $$2}')
LDFLAGS := -ldflags "-s -w -X $(MODULE)/pkg/version.Version=$(VERSION) -X $(MODULE)/pkg/version.Commit=$(COMMIT) -X $(MODULE)/pkg/version.Date=$(DATE)"

dev:
	go run ./cmd/barcode-server

server:
	go build $(LDFLAGS) -o bin/barcode-server ./cmd/barcode-server

build: server

web-build:
	cd web && pnpm install && pnpm run build

embed: web-build
	rm -rf cmd/barcode-server/dist && mkdir -p cmd/barcode-server/dist && cp -r web/dist/* cmd/barcode-server/dist/
	go build -tags embed_frontend $(LDFLAGS) -o bin/barcode-server ./cmd/barcode-server
	rm -rf cmd/barcode-server/dist

test:
	go test -v ./...

docker:
	docker build -t warehouse-barcode .

clean:
	rm -rf bin/ cmd/barcode-server/dist/
