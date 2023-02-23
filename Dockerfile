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

# Velero binary build section
FROM --platform=$BUILDPLATFORM golang:1.18-bullseye as velero-builder

ARG GOPROXY
ARG BIN
ARG PKG
ARG VERSION
ARG REGISTRY
ARG GIT_SHA
ARG GIT_TREE_STATE
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

ENV CGO_ENABLED=0 \
    GO111MODULE=on \
    GOPROXY=${GOPROXY} \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    GOARM=${TARGETVARIANT} \
    LDFLAGS="-X ${PKG}/pkg/buildinfo.Version=${VERSION} -X ${PKG}/pkg/buildinfo.GitSHA=${GIT_SHA} -X ${PKG}/pkg/buildinfo.GitTreeState=${GIT_TREE_STATE} -X ${PKG}/pkg/buildinfo.ImageRegistry=${REGISTRY}"

WORKDIR /go/src/github.com/vmware-tanzu/velero

COPY . /go/src/github.com/vmware-tanzu/velero

RUN mkdir -p /output/usr/bin && \
    export GOARM=$( echo "${GOARM}" | cut -c2-) && \
    go build -o /output/${BIN} \
    -ldflags "${LDFLAGS}" ${PKG}/cmd/${BIN}

# Restic binary build section
FROM --platform=$BUILDPLATFORM golang:1.19-bullseye as restic-builder

ARG BIN
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG RESTIC_VERSION

env CGO_ENABLED=0 \
    GO111MODULE=on \
    GOPROXY=${GOPROXY} \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    GOARM=${TARGETVARIANT}

COPY . /go/src/github.com/vmware-tanzu/velero

RUN mkdir -p /output/usr/bin && \
    export GOARM=$(echo "${GOARM}" | cut -c2-) && \
    /go/src/github.com/vmware-tanzu/velero/hack/build-restic.sh

# Velero image packing section
FROM gcr.io/distroless/base-debian11@sha256:99133cb0878bb1f84d1753957c6fd4b84f006f2798535de22ebf7ba170bbf434

LABEL maintainer="Nolan Brubaker <brubakern@vmware.com>"

COPY --from=velero-builder /output /

COPY --from=restic-builder /output /

USER nonroot:nonroot

