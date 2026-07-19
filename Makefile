# claude-hud-go build matrix.
#
# `make build-all` cross-compiles static binaries for every supported target
# into bin/. The plugin ships these; the /setup command picks the one matching
# the user's OS/arch.

VERSION ?= 0.1.0
LDFLAGS := -s -w -X github.com/jarrodwatts/claude-hud-go/internal/version.Version=$(VERSION)
PKG     := ./cmd/claude-hud
BIN     := bin

.PHONY: build build-all clean test tidy

# Host-native build (for local testing).
build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN)/claude-hud $(PKG)

# All six release targets. CGO disabled for fully static binaries.
build-all:
	@mkdir -p $(BIN)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN)/claude-hud-darwin-amd64      $(PKG)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BIN)/claude-hud-darwin-arm64      $(PKG)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN)/claude-hud-linux-amd64       $(PKG)
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BIN)/claude-hud-linux-arm64       $(PKG)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN)/claude-hud-windows-amd64.exe $(PKG)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BIN)/claude-hud-windows-arm64.exe $(PKG)

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -f $(BIN)/claude-hud*
