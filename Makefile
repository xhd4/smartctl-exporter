SHELL := /bin/sh

DIST_DIR := dist
ARCH ?= windows-amd64
SERVICE_NAME ?= smartctl-exporter
HOST_EXE ?= smartctl-exporter.exe
SMARTCTL_EXPORTER_VERSION ?= v0.14.0
PACKAGE_DIR := $(DIST_DIR)/$(ARCH)
ZIP_NAME := smartctl-exporter-$(ARCH).zip

ifeq ($(OS),Windows_NT)
	MKDIR_CMD := - mkdir $(PACKAGE_DIR)
	STOP_SERVICE_CMD := powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$(CURDIR)/scripts/stop-service.ps1" -ServiceName "$(SERVICE_NAME)" -DistDir "$(CURDIR)/$(PACKAGE_DIR)" -ExeName "$(HOST_EXE)"
	START_SERVICE_CMD := powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$(CURDIR)/scripts/start-service.ps1" -ServiceName "$(SERVICE_NAME)" -DistDir "$(CURDIR)/$(PACKAGE_DIR)" -ExeName "$(HOST_EXE)"
	CLEAN_DIR_CMD := powershell.exe -NoProfile -Command "if (Test-Path '$(CURDIR)/$(DIST_DIR)') { Remove-Item -Recurse -Force '$(CURDIR)/$(DIST_DIR)' -ErrorAction SilentlyContinue }"
	ZIP_CMD := powershell.exe -NoProfile -Command "Compress-Archive -Path '$(CURDIR)/$(PACKAGE_DIR)/smartctl-exporter.exe' -DestinationPath '$(CURDIR)/$(DIST_DIR)/$(ZIP_NAME)' -Force"
else
	MKDIR_CMD := mkdir -p $(PACKAGE_DIR)
	STOP_SERVICE_CMD := @echo "skip stop-service-win (non-Windows)"
	START_SERVICE_CMD := @echo "skip start-service-win (non-Windows)"
	CLEAN_DIR_CMD := rm -rf $(DIST_DIR)
	ZIP_CMD := (cd $(PACKAGE_DIR) && zip -q -r ../$(ZIP_NAME) smartctl-exporter.exe)
endif

DOCKER_BUILD := docker build \
	--target artifact \
	--build-arg SMARTCTL_EXPORTER_VERSION=$(SMARTCTL_EXPORTER_VERSION) \
	--output type=local,dest=./$(PACKAGE_DIR) \
	-f "$(CURDIR)/Dockerfile" \
	"$(CURDIR)"

.PHONY: help docker-build-host package-win docker-win clean

.DEFAULT_GOAL := help

help:
	@echo "smartctl-exporter — make targets"
	@echo ""
	@echo "  docker-build-host    Build $(HOST_EXE) via Docker into $(PACKAGE_DIR)/"
	@echo "  package-win          build + zip → $(DIST_DIR)/$(ZIP_NAME)"
	@echo "  docker-win           stop service, rebuild host, start service (Windows)"
	@echo "  clean                Remove $(DIST_DIR)/"
	@echo ""
	@echo "Variables: SMARTCTL_EXPORTER_VERSION ARCH"

docker-build-host:
	$(MKDIR_CMD)
	$(DOCKER_BUILD)

package-win: docker-build-host
	$(ZIP_CMD)
	@echo "Built $(DIST_DIR)/$(ZIP_NAME)"

docker-win: stop-service-win docker-build-host start-service-win

stop-service-win:
	$(STOP_SERVICE_CMD)

start-service-win:
	$(START_SERVICE_CMD)

clean:
	- $(CLEAN_DIR_CMD)
