SHELL := /bin/sh

DIST_DIR := dist
GOARCH ?= amd64
ARCH ?= windows-$(GOARCH)
SERVICE_NAME ?= smartctl-exporter
HOST_EXE ?= smartctl-exporter.exe
SMARTCTL_EXPORTER_VERSION ?= v0.14.0
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
PACKAGE_DIR := $(DIST_DIR)/$(ARCH)
OUT_EXE := $(DIST_DIR)/smartctl-exporter-$(GOARCH).exe

ifeq ($(OS),Windows_NT)
	MKDIR_CMD := - mkdir $(PACKAGE_DIR)
	STOP_SERVICE_CMD := powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$(CURDIR)/scripts/stop-service.ps1" -ServiceName "$(SERVICE_NAME)" -DistDir "$(CURDIR)/$(PACKAGE_DIR)" -ExeName "$(HOST_EXE)"
	START_SERVICE_CMD := powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$(CURDIR)/scripts/start-service.ps1" -ServiceName "$(SERVICE_NAME)" -DistDir "$(CURDIR)/$(PACKAGE_DIR)" -ExeName "$(HOST_EXE)"
	CLEAN_DIR_CMD := powershell.exe -NoProfile -Command "if (Test-Path '$(CURDIR)/$(DIST_DIR)') { Remove-Item -Recurse -Force '$(CURDIR)/$(DIST_DIR)' -ErrorAction SilentlyContinue }"
	COPY_OUT_CMD := powershell.exe -NoProfile -Command "Copy-Item -Force '$(CURDIR)/$(PACKAGE_DIR)/smartctl-exporter.exe' '$(CURDIR)/$(OUT_EXE)'"
else
	MKDIR_CMD := mkdir -p $(PACKAGE_DIR)
	STOP_SERVICE_CMD := @echo "skip stop-service-win (non-Windows)"
	START_SERVICE_CMD := @echo "skip start-service-win (non-Windows)"
	CLEAN_DIR_CMD := rm -rf $(DIST_DIR)
	COPY_OUT_CMD := cp $(PACKAGE_DIR)/smartctl-exporter.exe $(OUT_EXE)
endif

DOCKER_BUILD := docker build \
	--target artifact \
	--build-arg SMARTCTL_EXPORTER_VERSION=$(SMARTCTL_EXPORTER_VERSION) \
	--build-arg GIT_COMMIT=$(GIT_COMMIT) \
	--build-arg TARGET_GOARCH=$(GOARCH) \
	--output type=local,dest=./$(PACKAGE_DIR) \
	-f "$(CURDIR)/Dockerfile" \
	"$(CURDIR)"

.PHONY: help docker-build-host package-win docker-win clean versioninfo go-build-win

.DEFAULT_GOAL := help

help:
	@echo "smartctl-exporter — make targets"
	@echo ""
	@echo "  docker-build-host    Build $(HOST_EXE) via Docker into $(PACKAGE_DIR)/"
	@echo "  package-win          build → $(OUT_EXE)"
	@echo "  go-build-win         Local Go cross-build → $(OUT_EXE) (with VERSIONINFO)"
	@echo "  docker-win           stop service, rebuild host, start service (Windows)"
	@echo "  clean                Remove $(DIST_DIR)/"
	@echo ""
	@echo "Variables: SMARTCTL_EXPORTER_VERSION GIT_COMMIT GOARCH (amd64|arm64)"

versioninfo:
	@chmod +x "$(CURDIR)/scripts/gen-versioninfo.sh"
	"$(CURDIR)/scripts/gen-versioninfo.sh" \
		"$(SMARTCTL_EXPORTER_VERSION)" \
		"$(GIT_COMMIT)" \
		"$(GOARCH)" \
		"$(CURDIR)/cmd/smartctl-exporter/resource.syso"

go-build-win: versioninfo
	$(MKDIR_CMD)
	CGO_ENABLED=0 GOOS=windows GOARCH=$(GOARCH) go build -trimpath \
		-ldflags="-s -w -X main.version=$(SMARTCTL_EXPORTER_VERSION) -X main.commit=$(GIT_COMMIT)" \
		-o $(OUT_EXE) ./cmd/smartctl-exporter
	@echo "Built $(OUT_EXE)"

docker-build-host:
	$(MKDIR_CMD)
	$(DOCKER_BUILD)

package-win: docker-build-host
	$(COPY_OUT_CMD)
	@echo "Built $(OUT_EXE)"

docker-win: stop-service-win docker-build-host start-service-win

stop-service-win:
	$(STOP_SERVICE_CMD)

start-service-win:
	$(START_SERVICE_CMD)

clean:
	- $(CLEAN_DIR_CMD)
