# Copyright IBM Corp All Rights Reserved.
# Copyright London Stock Exchange Group All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0

GOTOOLS = counterfeiter golint goimports protoc-gen-go ginkgo gocov gocov-xml misspell mockery manifest-tool
BUILD_DIR ?= .build
GOTOOLS_GOPATH ?= $(BUILD_DIR)/gotools
GOTOOLS_BINDIR ?= $(GOPATH)/bin

# go tool->path mapping
go.fqp.counterfeiter := github.com/maxbrunsfeld/counterfeiter
go.fqp.gocov         := github.com/axw/gocov/gocov
go.fqp.gocov-xml     := github.com/AlekSi/gocov-xml
go.fqp.goimports     := golang.org/x/tools/cmd/goimports
go.fqp.golint        := golang.org/x/lint/golint
go.fqp.manifest-tool := github.com/estesp/manifest-tool
go.fqp.misspell      := github.com/client9/misspell/cmd/misspell
go.fqp.mockery       := github.com/vektra/mockery/cmd/mockery

.PHONY: gotools-install
gotools-install: $(patsubst %,$(GOTOOLS_BINDIR)/%, $(GOTOOLS))

.PHONY: gotools-clean
gotools-clean:
	-@rm -rf $(BUILD_DIR)/gotools

# Special override for protoc-gen-go since we want to use the version vendored with the project
gotool.protoc-gen-go:
	@echo "Building github.com/golang/protobuf/protoc-gen-go -> protoc-gen-go"
	GO111MODULE=off go get -d -u github.com/golang/protobuf/protoc-gen-go
	@git -C $(GOPATH)/src/github.com/golang/protobuf/protoc-gen-go checkout aa810b61a9c79d51363740d207bb46cf8e620ed5
	GO111MODULE=off GOBIN=$(abspath $(GOTOOLS_BINDIR)) go install github.com/golang/protobuf/protoc-gen-go

# Special override for ginkgo since we want to use the version vendored with the project
gotool.ginkgo:
	@echo "Building github.com/onsi/ginkgo/ginkgo -> ginkgo"
	GO111MODULE=off go get -d -u github.com/onsi/ginkgo/ginkgo
	@git -C $(GOPATH)/src/github.com/onsi/ginkgo/ginkgo checkout a3b6351eb1ff8e1bfa30b2f55d7e282186ed8fee
	GO111MODULE=off GOBIN=$(abspath $(GOTOOLS_BINDIR)) go install github.com/onsi/ginkgo/ginkgo

# Special override for goimports since we want to use the version vendored with the project
gotool.goimports:
	@echo "Building golang.org/x/tools/cmd/goimports -> goimports"
	GO111MODULE=off go get -d -u golang.org/x/tools/cmd/goimports
	@git -C $(GOPATH)/src/golang.org/x/tools/cmd/goimports checkout f60e5f99f0816fc2d9ecb338008ea420248d2943
	GO111MODULE=off GOBIN=$(abspath $(GOTOOLS_BINDIR)) go install golang.org/x/tools/cmd/goimports

# Special override for golint since we want to use the version vendored with the project
gotool.golint:
	@echo "Building golang.org/x/lint/golint -> golint"
	GO111MODULE=off go get -d -u golang.org/x/lint/golint
	@git -C $(GOPATH)/src/golang.org/x/lint/golint checkout c67002cb31c3a748b7688c27f20d8358b4193582
	GO111MODULE=off GOBIN=$(abspath $(GOTOOLS_BINDIR)) go install golang.org/x/lint/golint


# Default rule for gotools uses the name->path map for a generic 'go get' style build
gotool.%:
	$(eval TOOL = ${subst gotool.,,${@}})
	@echo "Building ${go.fqp.${TOOL}} -> $(TOOL)"
	@GO111MODULE=off GOPATH=$(abspath $(GOTOOLS_GOPATH)) GOBIN=$(abspath $(GOTOOLS_BINDIR)) go get ${go.fqp.${TOOL}}

$(GOTOOLS_BINDIR)/%:
	$(eval TOOL = ${subst $(GOTOOLS_BINDIR)/,,${@}})
	@$(MAKE) -f gotools.mk gotool.$(TOOL)
