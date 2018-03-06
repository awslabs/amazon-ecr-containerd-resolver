# Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You
# may not use this file except in compliance with the License. A copy of
# the License is located at
#
# 	http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is
# distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
# ANY KIND, either express or implied. See the License for the specific
# language governing permissions and limitations under the License.

ROOT := $(shell pwd)

all: build

SOURCEDIR=./
SOURCES := $(shell find $(SOURCEDIR) -name '*.go' | grep -v './vendor')
EXAMPLEDIR=$(SOURCEDIR)/example/ecr-pull
LOCAL_BINARY=$(ROOT)/bin/ecr-pull

VENDORDIR=$(ROOT)/vendor
DEP=Gopkg.toml
LOCK=Gopkg.lock


.PHONY: build
build: $(LOCAL_BINARY)

$(LOCAL_BINARY): $(SOURCES) $(VENDORDIR)
	cd $(EXAMPLEDIR) && go build -o $(LOCAL_BINARY) .

.PHONY: vendor
vendor: $(VENDORDIR)

$(VENDORDIR): $(DEP)
	which dep || go get -u github.com/golang/dep/cmd/dep
	dep ensure -vendor-only

.PHONY: test
test: $(SOURCES) $(VENDORDIR)
	go test -v $(shell go list ./... | grep -v '/vendor/')

.PHONY: clean
clean:
	@rm $(LOCAL_BINARY) ||:
