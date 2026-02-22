.PHONY: build build-all build-gui build-all-gui clean

# Force Bash shell (wichtig für Windows make)
SHELL := /bin/bash

# Detect OS
ifeq ($(OS),Windows_NT)
	BINARY_NAME=pings.exe
	GUI_BINARY_NAME=pings-gui.exe
else
	BINARY_NAME=pings
	GUI_BINARY_NAME=pings-gui
endif

# Lokaler Build für aktuelles System
build:
	go build -o $(BINARY_NAME) ./cmd/pings

# GUI Build für aktuelles System
# Auf Windows: kein Konsolenfenster dank -H windowsgui
ifeq ($(OS),Windows_NT)
build-gui:
	go build -ldflags "-H windowsgui" -o $(GUI_BINARY_NAME) ./cmd/pings-gui
else
build-gui:
	go build -o $(GUI_BINARY_NAME) ./cmd/pings-gui
endif

# Alle CLI-Plattformen bauen (kein CGO → Cross-Compilation funktioniert)
build-all:
	GOOS=windows GOARCH=amd64 go build -o build/pings-windows-amd64.exe ./cmd/pings
	GOOS=windows GOARCH=arm64 go build -o build/pings-windows-arm64.exe ./cmd/pings
	GOOS=linux   GOARCH=amd64 go build -o build/pings-linux-amd64 ./cmd/pings
	GOOS=linux   GOARCH=arm64 go build -o build/pings-linux-arm64 ./cmd/pings
	GOOS=darwin  GOARCH=amd64 go build -o build/pings-darwin-amd64 ./cmd/pings
	GOOS=darwin  GOARCH=arm64 go build -o build/pings-darwin-arm64 ./cmd/pings
	@echo "✅ All CLI builds in build/"

# GUI Build: muss auf dem Zielsystem selbst gebaut werden (Fyne braucht CGO)
# Linux:   sudo zypper install libX11-devel Mesa-libGL-devel libXrandr-devel libXcursor-devel libXinerama-devel libXi-devel libXxf86vm-devel
# Windows: MinGW-w64 installieren (z.B. via MSYS2 oder TDM-GCC), dann: make build-gui
# macOS:   Xcode Command Line Tools reichen: xcode-select --install
build-all-gui:
	go build -o build/pings-gui-$(shell go env GOOS)-$(shell go env GOARCH)$(if $(filter windows,$(shell go env GOOS)),.exe) ./cmd/pings-gui
	@echo "✅ GUI build in build/"

clean:
	rm -rf build/
	rm -f pings pings.exe pings-gui pings-gui.exe
