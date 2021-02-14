
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

debug:
	cd cmd/goneovim; \
	dlv debug --build-flags -race

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

