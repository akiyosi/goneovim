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
OSNAME=Windows_NT
else ifeq ($(shell uname), Darwin)
RUNTIME_DIR=$(DEPLOYMENT_DARWIN)/goneovim.app/Contents/Resources/
OSNAME=Darwin
else ifeq ($(shell uname), Linux)
RUNTIME_DIR=$(DEPLOYMENT_LINUX)/
OSNAME=Linux
endif

.PHONY: clean debug run build build-docker-linux build-docker-windows

# If the first argument is "run"...
ifeq (debug,$(firstword $(MAKECMDGOALS)))
  # use the rest as arguments for "run"
  DEBUG_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
  # ...and turn them into do-nothing targets
  $(eval $(DEBUG_ARGS):;@:)
endif

app:
	@export GO111MODULE=off ; \
	cd cmd/goneovim ; \
	test -f ../../editor/moc.go & qtmoc ; \
	qtdeploy -ldflags "-X github.com/akiyosi/goneovim/editor.Version=$(VERSION)" build desktop ; \
	cp -pR ../../runtime $(RUNTIME_DIR)
ifeq ($(OSNAME),Darwin)
	@/usr/libexec/PlistBuddy -c "Add :CFBundleVersion string $(VERSION_HASH)" "./cmd/goneovim/deploy/darwin/goneovim.app/Contents/Info.plist" ; \
	/usr/libexec/PlistBuddy -c "Add :CFBundleShortVersionString string $(VERSION)"  "./cmd/goneovim/deploy/darwin/goneovim.app/Contents/Info.plist" ; \
	cd cmd/goneovim/deploy/darwin/goneovim.app/Contents/Frameworks/ ; \
	rm -fr QtQuick.framework ; \
	rm -fr QtVirtualKeyboard.framework
endif

clean:
	@export GO111MODULE=off ; \
	rm -fr cmd/goneovim/deploy/* ; \
	rm -fr editor/*moc*

linux:
	@export GO111MODULE=off ; \
	cd cmd/goneovim ; \
	qtdeploy -docker build linux_static ; \
	cp -pR ../../runtime $(DEPLOYMENT_LINUX)

windows:
	@export GO111MODULE=off ; \
	cd cmd/goneovim ; \
	qtdeploy -docker build windows_64_static ; \
	cp -pR ../../runtime $(DEPLOYMENT_WINDOWS)

darwin:
	@export GO111MODULE=off ; \
	cd cmd/goneovim ; \
	qtdeploy -vagrant build darwin/darwin
	cp -pR ../../runtime $(DEPLOYMENT_WINDOWS)

debug:
	@export GO111MODULE=off ; \
	cd cmd/goneovim ; \
	test -f ../../editor/moc.go & qtmoc ; \
	dlv debug --build-flags -race -- $(DEBUG_ARGS)
