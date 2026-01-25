.PHONY: build build-all clean

# Detect OS
ifeq ($(OS),Windows_NT)
	BINARY_NAME=pings.exe
else
	BINARY_NAME=pings
endif

# Lokaler Build für aktuelles System
build:
	go build -o $(BINARY_NAME) .

# Alle Plattformen bauen (mit Plattform-Info im Namen für Releases)
build-all:
	GOOS=windows GOARCH=amd64 go build -o build/pings-windows-amd64.exe .
	GOOS=windows GOARCH=arm64 go build -o build/pings-windows-arm64.exe .
	GOOS=linux GOARCH=amd64 go build -o build/pings-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -o build/pings-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build -o build/pings-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -o build/pings-darwin-arm64 .
	@echo "✅ All builds in build/"

clean:
	rm -rf build/
	rm -f pings.exe pings
