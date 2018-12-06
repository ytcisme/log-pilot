# Git commit sha.
COMMIT := $(strip $(shell git rev-parse --short HEAD 2>/dev/null))
COMMIT := $(if $(COMMIT),$(COMMIT),"Unknown")

# Current version of the project.
VERSION ?= $(COMMIT)

# Docker image name.
TARGET := log-pilot
IMAGE := cargo.caicloudprivatetest.com/caicloud/$(TARGET):$(VERSION)

.PHONY: default build push

default: push

build:
	docker build -t $(IMAGE) .

push: build
	docker push $(IMAGE)
