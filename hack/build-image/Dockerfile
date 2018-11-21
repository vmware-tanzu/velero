# Copyright 2018 the Heptio Ark contributors.
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

FROM golang:1.11-alpine3.8

RUN apk add --update --no-cache git bash && \
    mkdir -p /go/src/k8s.io && \
    cd /go/src/k8s.io && \
    git config --global advice.detachedHead false && \
    git clone -b kubernetes-1.11.0 https://github.com/kubernetes/code-generator && \
    git clone -b kubernetes-1.11.0 https://github.com/kubernetes/apimachinery && \
    go get golang.org/x/tools/cmd/goimports && \
    cd /go/src/golang.org/x/tools && \
    git checkout 40a48ad93fbe707101afb2099b738471f70594ec && \
    go install ./cmd/goimports && \
    echo chmod -R a+w /go
