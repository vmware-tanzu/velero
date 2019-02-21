PROJ_NAME=Velero
JEKYLL_VERSION=3.5
IMAGE_NAME=doc-test

DIR := ${CURDIR}

IMAGE_TAG ?= latest
IMAGE = gcr.io/heptio-images/$(IMAGE_NAME)
GIT_REF ?= $(shell git rev-parse --short=8 --verify HEAD)

DOCKER ?= docker

image:
	$(DOCKER) build -t $(IMAGE):$(GIT_REF) -t $(IMAGE):$(IMAGE_TAG) .

push: image
	$(DOCKER) push $(IMAGE):$(IMAGE_TAG)
	$(DOCKER) push $(IMAGE):$(GIT_REF)

test: image
	docker run \
	-v $(DIR):/project \
	-e PROJ_NAME=$(PROJ_NAME) \
	gcr.io/heptio-images/$(IMAGE_NAME):latest

serve:
	docker run \
	--rm \
	-v $(DIR):/srv/jekyll \
	-it -p 4000:4000 \
	jekyll/jekyll:$(JEKYLL_VERSION) \
	jekyll serve

clean:
	rm -rf logs/