# Copyright 2018 The Kubernetes Authors.
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

IMAGE=registry.nubera.be:4567/docker/ovc-disk-csi-driver
VERSION=latest

.PHONY: ovc-csi-driver
ovc-csi-driver:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-X git.nubera.be/Customers/gig/csi.vendorVersion=${VERSION}" -o bin/ovc-csi-driver ./cmd/

.PHONY: test
test:
	go test -v -race ./pkg/...

.PHONY: test-sanity
test-sanity:
	go test -v ./tests/sanity/...

.PHONY: test-integration
test-integration:
	go test -c ./tests/integration/... -o bin/integration.test && \
	sudo -E bin/integration.test -ginkgo.v

.PHONY: image
image:
	docker build -t $(IMAGE):$(VERSION) .

.PHONY: push
push:
	docker push $(IMAGE):$(VERSION)