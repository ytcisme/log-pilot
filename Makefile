# Git commit sha.
COMMIT := $(strip $(shell git rev-parse --short HEAD 2>/dev/null))
COMMIT := $(if $(COMMIT),$(COMMIT),"Unknown")

# This repo's root import path (under GOPATH).
ROOT := github.com/caicloud/log-pilot

# Container registries. You can use multiple registries for a single project.
REGISTRIES ?= cargo.caicloudprivatetest.com/caicloud

# Project output directory.
OUTPUT_DIR := ./bin

# Build direcotory.
BUILD_DIR := ./build

# Project main package location (can be multiple ones).
CMD_DIR := ./cmd

# Current version of the project.
VERSION ?= $(COMMIT)

# Target binaries. You can build multiple binaries for a single project.
TARGETS := log-pilot

# Docker image name.
IMAGE := cargo.caicloudprivatetest.com/caicloud/log-pilot:$(VERSION)

.PHONY: default container push

default: push

build-linux:
	@for target in $(TARGETS); do                                                      \
	  for registry in $(REGISTRIES); do                                                \
	    docker run --rm                                                                \
	      -v ${PWD}:/go/src/$(ROOT)                                                    \
	      -w /go/src/$(ROOT)                                                           \
	      -e GOOS=linux                                                                \
	      -e GOARCH=amd64                                                              \
	      -e CGO_ENABLED=0                                                             \
	      -e GOPATH=/go                                                                \
	        $${registry}/golang:1.10-alpine3.6                                         \
	          go build -i -v -o $(OUTPUT_DIR)/$${target}                               \
	            -ldflags "-s -w -X $(ROOT)/pkg/version.Version=$(VERSION)              \
	            -X $(ROOT)/pkg/version.Commit=$(COMMIT)                                \
	            -X $(ROOT)/pkg/version.RepoRoot=$(ROOT)"                               \
	            ./$(CMD_DIR)/$${target};                                               \
	  done                                                                             \
	done

container: build-linux
	docker build -t $(IMAGE) -f $(BUILD_DIR)/log-pilot/Dockerfile .

push: container
	docker push $(IMAGE)
