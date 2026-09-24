APP_NAME := MP3 Renamer
BIN_NAME := mp3renamer
APP_ID   := is.ankeri.mp3renamer
VERSION  ?= 1.0.0.1
DIST     := dist

GOGIO := go run gioui.org/cmd/gogio@v0.10.0

MAC_APP := $(DIST)/macos/$(APP_NAME).app

.PHONY: help run build icon macos macos-arm64 macos-amd64 windows linux all clean

help:
	@echo "Targets:"
	@echo "  run            Run the app from source"
	@echo "  build          Build a binary for the current platform into $(DIST)/"
	@echo "  icon           Regenerate icon.png"
	@echo "  macos          Universal (Apple Silicon + Intel) .app bundle"
	@echo "  macos-arm64    Apple Silicon .app bundle"
	@echo "  macos-amd64    Intel .app bundle"
	@echo "  windows        Windows .exe (amd64 and arm64) with icon"
	@echo "  linux          Linux binary (must be run on Linux)"
	@echo "  all            macos + windows"
	@echo "  clean          Remove $(DIST)/"

run:
	go run .

build:
	go build -o $(DIST)/$(BIN_NAME) .

icon:
	go generate ./...

# ---- macOS ----

# gogio marks the bundle as a generic bundle (BNDL); patch it to an
# application, then re-sign ad hoc since editing Info.plist breaks the signature.
define finish_mac_app
	plutil -replace CFBundlePackageType -string APPL "$(1)/Contents/Info.plist"
	plutil -replace CFBundleName -string "$(APP_NAME)" "$(1)/Contents/Info.plist"
	codesign --force --deep --sign - "$(1)"
endef

macos-arm64 macos-amd64: macos-%:
	rm -rf "$(DIST)/macos-$*"
	mkdir -p "$(DIST)/macos-$*"
	$(GOGIO) -target macos -arch $* -appid $(APP_ID) -icon icon.png -o "$(DIST)/macos-$*/$(APP_NAME).app" .
	$(call finish_mac_app,$(DIST)/macos-$*/$(APP_NAME).app)

# Build both architectures and merge the executables with lipo.
macos: macos-arm64 macos-amd64
	rm -rf "$(DIST)/macos"
	mkdir -p "$(DIST)/macos"
	cp -R "$(DIST)/macos-arm64/$(APP_NAME).app" "$(MAC_APP)"
	lipo -create \
		"$(DIST)/macos-arm64/$(APP_NAME).app/Contents/MacOS/$(APP_NAME)" \
		"$(DIST)/macos-amd64/$(APP_NAME).app/Contents/MacOS/$(APP_NAME)" \
		-output "$(MAC_APP)/Contents/MacOS/$(APP_NAME)"
	$(call finish_mac_app,$(MAC_APP))
	@echo "Built $(MAC_APP)"

# ---- Windows ----

# Gio needs no cgo on Windows, so this cross-compiles from any OS.
windows:
	mkdir -p $(DIST)/windows
	$(GOGIO) -target windows -arch amd64 -version $(VERSION) -icon icon.png -o $(DIST)/windows/$(BIN_NAME)-amd64.exe .
	$(GOGIO) -target windows -arch arm64 -version $(VERSION) -icon icon.png -o $(DIST)/windows/$(BIN_NAME)-arm64.exe .
	@# gogio leaves resource files in the package dir; remove them so they
	@# don't get linked into later builds.
	rm -f *.syso

# ---- Linux ----

# Gio uses cgo for X11/Wayland on Linux, so build on a Linux machine.
linux:
	@if [ "$$(uname -s)" != "Linux" ]; then \
		echo "The linux target must be built on Linux (Gio needs cgo there)."; exit 1; \
	fi
	mkdir -p $(DIST)/linux
	go build -ldflags="-s -w" -o $(DIST)/linux/$(BIN_NAME) .

all: macos windows

clean:
	rm -rf $(DIST)
