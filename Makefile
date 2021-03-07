
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

.PHONY: debug build build-docker-linux build-docker-windows

# If the first argument is "run"...
ifeq (debug,$(firstword $(MAKECMDGOALS)))
  # use the rest as arguments for "run"
  DEBUG_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
  # ...and turn them into do-nothing targets
  $(eval $(DEBUG_ARGS):;@:)
endif

debug:
	cd cmd/goneovim; \
	dlv debug --build-flags -race -- $(DEBUG_ARGS)

build:
	cd cmd/goneovim; \
	qtdeploy build desktop; \
	cp -pR ../../runtime $(RUNTIME_DIR)

build-docker-linux:
	cd cmd/goneovim; \
	qtdeploy -docker build linux; \
	cp -pR ../../runtime $(DEPLOYMENT_LINUX)

build-docker-windows:
	cd cmd/goneovim; \
	qtdeploy -docker build akiyosi/qt:windows_64_shared_msvc_512; \
	cp -pR ../../runtime $(DEPLOYMENT_WINDOWS)

