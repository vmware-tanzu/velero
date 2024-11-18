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
ARG GOEXPERIMENT

ENV CGO_ENABLED=0 \
    GO111MODULE=on \
    GOPROXY=${GOPROXY} \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    GOARM=${TARGETVARIANT} \
    GOEXPERIMENT=${GOEXPERIMENT} \
    LDFLAGS="-X ${PKG}/pkg/buildinfo.Version=${VERSION} -X ${PKG}/pkg/buildinfo.GitSHA=${GIT_SHA} -X ${PKG}/pkg/buildinfo.GitTreeState=${GIT_TREE_STATE} -X ${PKG}/pkg/buildinfo.ImageRegistry=${REGISTRY}"

WORKDIR /go/src/github.com/vmware-tanzu/velero
# verifies go cli has boring
RUN if [ "${GOEXPERIMENT}" = "boringcrypto" ]; then \
        go tool nm $(which go) | grep sig.BoringCrypto \
    fi

COPY . /go/src/github.com/vmware-tanzu/velero
RUN mkdir -p /output/usr/bin && \
    export GOARM=$( echo "${GOARM}" | cut -c2-) && \
    go build -o /output/${BIN} \
    -ldflags "${LDFLAGS}" ${PKG}/cmd/${BIN} && \
    go build -o /output/velero-helper \
    -ldflags "${LDFLAGS}" ${PKG}/cmd/velero-helper && \
    go clean -modcache -cache

# Restic binary build section
FROM --platform=$BUILDPLATFORM golang:1.22-bookworm AS restic-builder

ARG GOPROXY
ARG BIN
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG RESTIC_VERSION
ARG GOEXPERIMENT

ENV CGO_ENABLED=0 \
    GO111MODULE=on \
    GOPROXY=${GOPROXY} \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    GOARM=${TARGETVARIANT} \
    GOEXPERIMENT=${GOEXPERIMENT}
# verifies go cli has boring
RUN if [ "${GOEXPERIMENT}" = "boringcrypto" ]; then \
        go tool nm $(which go) | grep sig.BoringCrypto \
    fi

COPY . /go/src/github.com/vmware-tanzu/velero

RUN mkdir -p /output/usr/bin && \
    export GOARM=$(echo "${GOARM}" | cut -c2-) && \
    /go/src/github.com/vmware-tanzu/velero/hack/build-restic.sh && \
    go clean -modcache -cache

# validate that FIPS is enabled in the binaries
FROM --platform=$BUILDPLATFORM golang:1.22-bookworm AS fips-validator
ARG GOEXPERIMENT
ARG BIN
COPY --from=velero-builder /output /
COPY --from=restic-builder /output /
RUN if [ "${GOEXPERIMENT}" = "boringcrypto" ]; then \
        go tool nm ${BIN} > ${BIN}nm && \
        go tool nm velero-helper > velero-helpernm && \
        go tool nm restic > resticnm && \
        grep ${BIN}nm -qe sig.BoringCrypto && \
        grep velero-helpernm -qe sig.BoringCrypto && \
        grep resticnm -qe sig.BoringCrypto && \
    fi

# Velero image packing section
FROM paketobuildpacks/run-jammy-tiny:latest

LABEL maintainer="Xun Jiang <jxun@vmware.com>"

COPY --from=velero-builder /output /

COPY --from=restic-builder /output /

USER cnb:cnb

