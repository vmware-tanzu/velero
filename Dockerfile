# Copyright 2020 the Velero contributors.
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
FROM --platform=$BUILDPLATFORM golang:1.16 as builder-env

ARG GOPROXY
ARG PKG
ARG VERSION
ARG GIT_SHA
ARG GIT_TREE_STATE
ARG REGISTRY

ENV CGO_ENABLED=0 \
    GO111MODULE=on \
    GOPROXY=${GOPROXY}

WORKDIR /go/src/github.com/vmware-tanzu/velero

RUN apt-get update && apt-get install -y bzip2

COPY go.mod go.sum /go/src/github.com/vmware-tanzu/velero
RUN go mod download

FROM --platform=$BUILDPLATFORM builder-env as builder

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG PKG
ARG BIN
ARG RESTIC_VERSION

WORKDIR /go/src/github.com/vmware-tanzu/velero

COPY hack /go/src/github.com/vmware-tanzu/velero/hack

ENV GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    GOARM=${TARGETVARIANT}

RUN mkdir -p /output/usr/bin && \
    bash ./hack/download-restic.sh

ENV LDFLAGS="-X ${PKG}/pkg/buildinfo.Version=${VERSION} -X ${PKG}/pkg/buildinfo.GitSHA=${GIT_SHA} -X ${PKG}/pkg/buildinfo.GitTreeState=${GIT_TREE_STATE} -X ${PKG}/pkg/buildinfo.ImageRegistry=${REGISTRY}"

COPY . /go/src/github.com/vmware-tanzu/velero
RUN export GOARM=$( echo "${GOARM}" | cut -c2-) && \
    go build -o /output/${BIN} \
    -ldflags "${LDFLAGS}" ${PKG}/cmd/${BIN}

FROM gcr.io/distroless/base-debian10:nonroot

LABEL maintainer="Nolan Brubaker <brubakern@vmware.com>"

COPY --from=builder /output /

USER nonroot:nonroot

