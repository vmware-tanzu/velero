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
FROM --platform=$BUILDPLATFORM golang:1.22-bookworm AS velero-builder

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

RUN mkdir -p /output/usr/bin
WORKDIR /go/src/github.com/vmware-tanzu/velero
COPY go.mod go.sum /go/src/github.com/vmware-tanzu/velero/
# --mount=type=cache,target=/go/pkg/mod,id=vbb allows reuse of build cache across builds instead of invalidating whole cache when go.mod changes
# id is to allow other stages to use the same cache path without conflicting with this stage.
# velero-builder and restic-builder uses a different go.mod from each other. id helps to avoid sharing cache with velero-builder.
RUN --mount=type=cache,target=/go/pkg/mod,id=vbb go mod download
COPY . /go/src/github.com/vmware-tanzu/velero
RUN --mount=type=cache,target=/go/pkg/mod,id=vbb GOARM=$( echo "${GOARM}" | cut -c2-) go build -o /output/${BIN} \
    -ldflags "${LDFLAGS}" ${PKG}/cmd/${BIN}
RUN --mount=type=cache,target=/go/pkg/mod,id=vbb GOARM=$( echo "${GOARM}" | cut -c2-) go build -o /output/velero-helper \
    -ldflags "${LDFLAGS}" ${PKG}/cmd/velero-helper

# Restic binary build section
FROM --platform=$BUILDPLATFORM golang:1.22-bookworm AS restic-builder

ARG GOPROXY
ARG BIN
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG RESTIC_VERSION

ENV CGO_ENABLED=0 \
    GO111MODULE=on \
    GOPROXY=${GOPROXY} \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    GOARM=${TARGETVARIANT}

# /output dir needed by last stage to copy even when BIN is not velero
RUN mkdir -p /output/usr/bin && mkdir -p /build/restic
WORKDIR /build/restic

# cache go mod download before applying patches
RUN --mount=type=cache,target=/go/pkg/mod,id=restic if [ "${BIN}" = "velero" ]; then \
        git clone --single-branch -b v${RESTIC_VERSION} https://github.com/restic/restic.git . && \
        go mod download; \
    fi

# invalidate cache if patch changes
COPY hack/fix_restic_cve.txt /go/src/github.com/vmware-tanzu/velero/hack/

# cache go mod download after applying patches
RUN --mount=type=cache,target=/go/pkg/mod,id=restic if [ "${BIN}" = "velero" ]; then \
        git apply /go/src/github.com/vmware-tanzu/velero/hack/fix_restic_cve.txt && \
        go mod download; \
    fi

# arch specific build layer
RUN --mount=type=cache,target=/go/pkg/mod,id=restic if [ "${BIN}" = "velero" ]; then \ 
        GOARM=$(echo "${GOARM}" | cut -c2-) go run build.go --goos "${GOOS}" --goarch "${GOARCH}" --goarm "${GOARM}" -o /output/usr/bin/restic && \
        chmod +x /output/usr/bin/restic; \
    fi

# Velero image packing section
FROM paketobuildpacks/run-jammy-tiny:latest

LABEL maintainer="Xun Jiang <jxun@vmware.com>"

COPY --from=velero-builder /output /

COPY --from=restic-builder /output /

USER cnb:cnb

