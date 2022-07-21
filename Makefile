TAG := $(shell git describe --tags --abbrev=0)
VERSION := $(shell git describe --tags)
VERSION_HASH := $(shell git rev-parse HEAD)


# deployment directory
DEPLOYMENT_WINDOWS:=deploy/windows
DEPLOYMENT_DARWIN:=deploy/darwin
DEPLOYMENT_LINUX:=deploy/linux

# runtime directory
ifeq ($(OS),Windows_NT)
RUNTIME_DIR=$(DEPLOYMENT_WINDOWS)/
else ifeq ($(shell uname), Darwin)
RUNTIME_DIR=$(DEPLOYMENT_DARWIN)/goneovim.app/Contents/Resources/
else ifeq ($(shell uname), Linux)
RUNTIME_DIR=$(DEPLOYMENT_LINUX)/
endif

.PHONY: clean debug run build build-docker-linux build-docker-windows

# If the first argument is "run"...
ifeq (debug,$(firstword $(MAKECMDGOALS)))
  # use the rest as arguments for "run"
  DEBUG_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
  # ...and turn them into do-nothing targets
  $(eval $(DEBUG_ARGS):;@:)
endif

all: moc build

release: build-alldist-in-darwin rename archive-in-darwin

build-alldist-in-darwin: moc build build-docker-linux

rename:
	@cd cmd/goneovim/deploy ; \
	mv darwin Goneovim-$(TAG)-macos ;\
	mv linux Goneovim-$(TAG)-linux

archive-in-darwin:
	@cd cmd/goneovim/deploy ; \
	tar jcvf Goneovim-$(TAG)-macos.tar.bz2 Goneovim-$(TAG)-macos ;\
	tar jcvf Goneovim-$(TAG)-linux.tar.bz2 Goneovim-$(TAG)-linux

moc:
	@export export GO111MODULE=off ; \
	rm -fr editor/moc* ; \
	cd cmd/goneovim ; \
	qtmoc

build:
	@export GO111MODULE=off ; \
	cd cmd/goneovim ; \
	test -f ../../editor/moc.go & qtmoc ; \
	qtdeploy -ldflags "-X github.com/akiyosi/goneovim/editor.Version=$(VERSION)" build desktop ; \
	cp -pR ../../runtime $(RUNTIME_DIR)
	@if [[ $(shell uname) == 'Darwin' ]]; then \
	  /usr/libexec/PlistBuddy -c "Add :CFBundleVersion string $(VERSION_HASH)" "./cmd/goneovim/deploy/darwin/goneovim.app/Contents/Info.plist" ; \
	  /usr/libexec/PlistBuddy -c "Add :CFBundleShortVersionString string $(VERSION)"  "./cmd/goneovim/deploy/darwin/goneovim.app/Contents/Info.plist" ; \
          cd cmd/goneovim/deploy/darwin/goneovim.app/Contents/Frameworks/ ; \
          rm -fr QtQuick.framework ; \
          rm -fr QtVirtualKeyboard.framework ; \
	fi

debug:
	@export GO111MODULE=off ; \
	cd cmd/goneovim ; \
	test -f ../../editor/moc.go & qtmoc ; \
	dlv debug --build-flags -race -- $(DEBUG_ARGS)

run:
	@export GO111MODULE=off ; \
	cd cmd/goneovim; \
	test -f ../../editor/moc.go & qtmoc ; \
	go run main.go

clean:
	@export GO111MODULE=off ; \
	rm -fr cmd/goneovim/deploy/* ; \
	rm -fr editor/*moc*

build-linux:
	@export GO111MODULE=off ; \
	cd cmd/goneovim ; \
	qtdeploy -docker build linux ; \
	cp -pR ../../runtime $(DEPLOYMENT_LINUX)

build-windows:
	@export GO111MODULE=off ; \
	cd cmd/goneovim ; \
	qtdeploy -docker build windows_64_static ; \
	cp -pR ../../runtime $(DEPLOYMENT_WINDOWS)

release:

