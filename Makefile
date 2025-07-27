TAG := $(shell git describe --tags --abbrev=0)
VERSION := $(shell git describe --tags)
VERSION_HASH := $(shell git rev-parse HEAD)

# deployment directory
DEPLOYMENT_WINDOWS:=cmd/goneovim/deploy/windows
DEPLOYMENT_DARWIN:=cmd/goneovim/deploy/darwin
DEPLOYMENT_LINUX:=cmd/goneovim/deploy/linux
DEPLOYMENT_FREEBSD:=cmd/goneovim/deploy/freebsd

# runtime directory
ifeq ($(OS),Windows_NT)
RUNTIME_DIR=$(DEPLOYMENT_WINDOWS)/
OSNAME=Windows
else ifeq ($(shell uname), Darwin)
RUNTIME_DIR=$(DEPLOYMENT_DARWIN)/goneovim.app/Contents/Resources/
OSNAME=Darwin
else ifeq ($(shell uname), Linux)
RUNTIME_DIR=$(DEPLOYMENT_LINUX)/
OSNAME=Linux
else ifeq ($(shell uname), FreeBSD)
RUNTIME_DIR=$(DEPLOYMENT_FREEBSD)/
OSNAME=FreeBSD
endif

# qt bindings cmd
GOQTSETUP:=$(shell go env GOPATH)/bin/qtsetup
GOQTMOC:=$(shell go env GOPATH)/bin/qtmoc
GOQTDEPLOY:=$(shell go env GOPATH)/bin/qtdeploy

.PHONY: app qt_bindings clean linux windows darwin debug test help

# If the first argument is "run"...
ifeq (debug,$(firstword $(MAKECMDGOALS)))
  # use the rest as arguments for "run"
  DEBUG_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
  # ...and turn them into do-nothing targets
  $(eval $(DEBUG_ARGS):;@:)
endif


app: ## Build goneovim
	@test -f ./editor/moc.go && $(GOQTMOC) desktop ./cmd/goneovim && \
	go generate && \
	$(GOQTDEPLOY) build desktop ./cmd/goneovim && \
	cp -pR runtime $(RUNTIME_DIR)
ifeq ($(OSNAME),Darwin)
	@/usr/libexec/PlistBuddy -c "Add :CFBundleVersion string $(VERSION_HASH)" "./cmd/goneovim/deploy/darwin/goneovim.app/Contents/Info.plist" && \
	/usr/libexec/PlistBuddy -c "Add :CFBundleShortVersionString string $(VERSION)"  "./cmd/goneovim/deploy/darwin/goneovim.app/Contents/Info.plist"
	@if [ -d "./cmd/goneovim/deploy/darwin/goneovim.app/Contents/Frameworks/" ]; then \
	cd cmd/goneovim/deploy/darwin/goneovim.app/Contents/Frameworks/ && \
	rm -fr QtQuick.framework ; \
	rm -fr QtVirtualKeyboard.framework; \
	else \
	exit 0; \
	fi
endif

qt_bindings: ## Setup Qt bindings for Go.
	@version=v0.0.0-20240304155940-b43fff373ad5
	go get -d github.com/akiyosi/qt@$$version  && \
	go install -tags=no_env github.com/akiyosi/qt/cmd/...@$$version && \
	$(GOQTSETUP) -test=false && \
	go mod tidy
	go mod vendor

deps: ## Get dependent libraries.
	@go get github.com/akiyosi/goneovim
	@$(GOQTMOC) desktop ./cmd/goneovim

test: ## Test goneovim
	@go generate && go test ./editor

clean: ## Delete pre-built application binaries and Moc files.
	@rm -fr cmd/goneovim/deploy/*
	@rm -fr editor/*moc*

linux: ## Build binaries for Linux using Docker.
	@go generate && \
	cd cmd/goneovim && \
	$(GOQTDEPLOY) -docker build linux_static && \
	cp -pR ../../runtime $(DEPLOYMENT_LINUX)

windows: ## Build binaries for Windows using Docker.
	@go generate && \
	cd cmd/goneovim && \
	$(GOQTDEPLOY) -docker build windows_64_static && \
	cp -pR ../../runtime $(DEPLOYMENT_WINDOWS)

darwin: ## Build binaries for MacOS using Vagrant.
	@go generate && \
	cd cmd/goneovim && \
	$(GOQTDEPLOY) -vagrant build darwin/darwin && \
	cp -pR ../../runtime $(DEPLOYMENT_WINDOWS)

debug: ## Debug runs of the application using delve.
	@test -f ./editor/moc.go && $(GOQTMOC) desktop ./cmd/goneovim && \
	cd cmd/goneovim && \
	dlv debug --output goneovim --build-flags -race -- $(DEBUG_ARGS)

help: ## Display this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "%-20s %s\n", $$1, $$2}'
