# Copyright 2016 The Kubernetes Authors.
#
# Modifications Copyright 2017 the Heptio Ark contributors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# The binary to build (just the basename).
BIN := ark

# This repo's root import path (under GOPATH).
PKG := github.com/heptio/ark

# Where to push the docker image.
REGISTRY ?= gcr.io/heptio-images

# Which architecture to build - see $(ALL_ARCH) for options.
ARCH ?= linux-amd64

VERSION ?= master

###
### These variables should not need tweaking.
###

SRC_DIRS := cmd pkg # directories which hold app source (not vendored)

CLI_PLATFORMS := linux-amd64 linux-arm linux-arm64 darwin-amd64 windows-amd64
CONTAINER_PLATFORMS := linux-amd64 linux-arm linux-arm64

platform_temp = $(subst -, ,$(ARCH))
GOOS = $(word 1, $(platform_temp))
GOARCH = $(word 2, $(platform_temp))

# TODO(ncdc): support multiple image architectures once gcr.io supports manifest lists
# Set default base image dynamically for each arch
ifeq ($(GOARCH),amd64)
		DOCKERFILE ?= Dockerfile.alpine
endif
#ifeq ($(GOARCH),arm)
#		DOCKERFILE ?= Dockerfile.arm #armel/busybox
#endif
#ifeq ($(GOARCH),arm64)
#		DOCKERFILE ?= Dockerfile.arm64 #aarch64/busybox
#endif

IMAGE := $(REGISTRY)/$(BIN)

# If you want to build all binaries, see the 'all-build' rule.
# If you want to build all containers, see the 'all-container' rule.
# If you want to build AND push all containers, see the 'all-push' rule.
all: build

build-%:
	@$(MAKE) --no-print-directory ARCH=$* build

#container-%:
#	@$(MAKE) --no-print-directory ARCH=$* container

#push-%:
#	@$(MAKE) --no-print-directory ARCH=$* push

all-build: $(addprefix build-, $(CLI_PLATFORMS))

#all-container: $(addprefix container-, $(CONTAINER_PLATFORMS))

#all-push: $(addprefix push-, $(CONTAINER_PLATFORMS))

build: _output/bin/$(GOOS)/$(GOARCH)/$(BIN)

_output/bin/$(GOOS)/$(GOARCH)/$(BIN): build-dirs
	@echo "building: $@"
	$(MAKE) shell CMD="-c '\
		GOOS=$(GOOS) \
		GOARCH=$(GOARCH) \
		VERSION=$(VERSION) \
		PKG=$(PKG) \
		BIN=$(BIN) \
		OUTPUT_DIR=/output/$(GOOS)/$(GOARCH) \
		./hack/build.sh'"

TTY := $(shell tty -s && echo "-t")

BUILDER_IMAGE := ark-builder

# Example: make shell CMD="date > datefile"
shell: build-dirs build-image
	@docker run \
		-i $(TTY) \
		--rm \
		-u $$(id -u):$$(id -g) \
		-v "$$(pwd)/.go/pkg:/go/pkg" \
		-v "$$(pwd)/.go/std:/go/std" \
		-v "$$(pwd):/go/src/$(PKG)" \
		-v "$$(pwd)/_output/bin:/output" \
		-v "$$(pwd)/.go/std/$(GOOS)/$(GOARCH):/usr/local/go/pkg/$(GOOS)_$(GOARCH)_static" \
		-w /go/src/$(PKG) \
		$(BUILDER_IMAGE) \
		/bin/sh $(CMD)

DOTFILE_IMAGE = $(subst :,_,$(subst /,_,$(IMAGE))-$(VERSION))

container: verify test .container-$(DOTFILE_IMAGE) container-name
.container-$(DOTFILE_IMAGE): _output/bin/$(GOOS)/$(GOARCH)/$(BIN) $(DOCKERFILE)
	@cp $(DOCKERFILE) _output/.dockerfile-$(GOOS)-$(GOARCH)
	@docker build -t $(IMAGE):$(VERSION) -f _output/.dockerfile-$(GOOS)-$(GOARCH) _output
	@docker images -q $(IMAGE):$(VERSION) > $@

container-name:
	@echo "container: $(IMAGE):$(VERSION)"

push: .push-$(DOTFILE_IMAGE) push-name
.push-$(DOTFILE_IMAGE): .container-$(DOTFILE_IMAGE)
	@docker push $(IMAGE):$(VERSION)
	@if git describe --tags --exact-match >/dev/null 2>&1; \
	then \
		docker tag $(IMAGE):$(VERSION) $(IMAGE):latest; \
		docker push $(IMAGE):latest; \
	fi
	@docker images -q $(IMAGE):$(VERSION) > $@

push-name:
	@echo "pushed: $(IMAGE):$(VERSION)"

SKIP_TESTS ?=
test: build-dirs
ifneq ($(SKIP_TESTS), 1)
	@$(MAKE) shell CMD="-c 'hack/test.sh $(SRC_DIRS)'"
endif

verify:
ifneq ($(SKIP_TESTS), 1)
	@$(MAKE) shell CMD="-c 'hack/verify-all.sh'"
endif

update:
	@$(MAKE) shell CMD="-c 'hack/update-all.sh'"

release: all-tar-bin checksum

checksum:
	@cd _output/release; \
	sha256sum *.tar.gz > CHECKSUM; \
	cat CHECKSUM; \
	sha256sum CHECKSUM

all-tar-bin: $(addprefix tar-bin-, $(CLI_PLATFORMS))

tar-bin-%:
	$(MAKE) ARCH=$* VERSION=$(VERSION) tar-bin

GIT_DESCRIBE = $(shell git describe --tags --always --dirty)
tar-bin: build
	mkdir -p _output/release

# We do the subshell & wildcard ls so we can pick up $(BIN).exe for windows
	(cd _output/bin/$(GOOS)/$(GOARCH) && ls $(BIN)*) | \
		tar \
			-C _output/bin/$(GOOS)/$(GOARCH) \
			--files-from=- \
			-zcf _output/release/$(BIN)-$(GIT_DESCRIBE)-$(GOOS)-$(GOARCH).tar.gz

build-dirs:
	@mkdir -p _output/bin/$(GOOS)/$(GOARCH)
	@mkdir -p .go/src/$(PKG) .go/pkg .go/bin .go/std/$(GOOS)/$(GOARCH)

build-image:
	cd hack/build-image && docker build -t $(BUILDER_IMAGE) .

clean:
	rm -rf .container-* _output/.dockerfile-* .push-*
	rm -rf .go _output
	docker rmi $(BUILDER_IMAGE)

ci: build verify test
