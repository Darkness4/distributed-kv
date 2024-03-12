GO_SRCS := $(shell find . -type f -name '*.go' -a ! \( -name 'zz_generated*' -o -name '*_test.go' \))
GO_TESTS := $(shell find . -type f -name '*_test.go')
TAG_NAME = $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
TAG_NAME_DEV = $(shell git describe --tags --abbrev=0 2>/dev/null)
GIT_COMMIT = $(shell git rev-parse --short=7 HEAD)
VERSION = $(or $(TAG_NAME),$(and $(TAG_NAME_DEV),$(TAG_NAME_DEV)-dev),$(GIT_COMMIT))

###############################################################################
# Tools Finder
###############################################################################

# Go linter
golint := $(shell which golangci-lint)
ifeq ($(golint),)
golint := $(shell go env GOPATH)/bin/golangci-lint
endif

# Protobuf code generator
protoc-gen-go := $(shell which protoc-gen-go)
ifeq ($(protoc-gen-go),)
protoc-gen-go := $(shell go env GOPATH)/bin/protoc-gen-go
endif

# Connect RPC
protoc-gen-connect-go := $(shell which protoc-gen-connect-go)
ifeq ($(protoc-gen-connect-go),)
protoc-gen-connect-go := $(shell go env GOPATH)/bin/protoc-gen-connect-go
endif

# Buf (protobuf manager and linter)
buf := $(shell which buf)
ifeq ($(buf),)
buf := $(shell go env GOPATH)/bin/buf
endif

# CFSSL (Certificate Authority manager)
cfssl := $(shell which cfssl)
ifeq ($(cfssl),)
cfssl := $(shell go env GOPATH)/bin/cfssl
endif

# CFSSL JSON (JSON parser for CFSSL)
cfssljson := $(shell which cfssljson)
ifeq ($(cfssljson),)
cfssljson := $(shell go env GOPATH)/bin/cfssljson
endif

###############################################################################
# Build
###############################################################################

all: bin/dkv bin/dkvctl

bin/dkv: $(GO_SRCS)
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o "$@" ./cmd/dkv/main.go

bin/dkvctl: $(GO_SRCS)
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o "$@" ./cmd/dkvctl/main.go

###############################################################################
# Install missing tools
###############################################################################

$(golint):
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

$(protoc-gen-go):
	go install google.golang.org/protobuf/cmd/protoc-gen-go

$(protoc-gen-connect-go):
	go install connectrpc.com/connect/cmd/protoc-gen-connect-go

$(buf):
	go install github.com/bufbuild/buf/cmd/buf@latest

$(cfssl):
	go install github.com/cloudflare/cfssl/cmd/cfssl@latest

$(cfssljson):
	go install github.com/cloudflare/cfssl/cmd/cfssljson@latest

###############################################################################
# TLS
###############################################################################

certs/ca/tls.crt certs/ca/tls.key: $(cfssl) $(cfssljson)
	@umask 077
	$(cfssl) gencert -initca certs/ca/csr.json | cfssljson -bare ca
	mv ca.pem certs/ca/tls.crt
	mv ca-key.pem certs/ca/tls.key
	rm ca.csr

certs/peer/tls.crt certs/peer/tls.key: certs/ca/tls.crt certs/ca/tls.key $(cfssl) $(cfssljson)
	@umask 077
	$(cfssl) gencert \
		-ca=certs/ca/tls.crt \
		-ca-key=certs/ca/tls.key \
		-config=certs/ca/config.json \
		-profile=peer \
		certs/peer/csr.json | cfssljson -bare peer
	mv peer.pem certs/peer/tls.crt
	mv peer-key.pem certs/peer/tls.key
	rm peer.csr

certs/client/tls.crt certs/client/tls.key: certs/ca/tls.crt certs/ca/tls.key $(cfssl) $(cfssljson)
	@umask 077
	$(cfssl) gencert \
		-ca=certs/ca/tls.crt \
		-ca-key=certs/ca/tls.key \
		-config=certs/ca/config.json \
		-profile=client \
		certs/client/csr.json | cfssljson -bare client
	mv client.pem certs/client/tls.crt
	mv client-key.pem certs/client/tls.key
	rm client.csr

.PHONY: certs
certs: certs/ca/tls.crt certs/ca/tls.key certs/peer/tls.crt certs/peer/tls.key certs/client/tls.crt certs/client/tls.key

###############################################################################
# Various commands
###############################################################################

.PHONY: unit
unit:
	go test -race -covermode=atomic -tags=unit -timeout=30s ./...

.PHONY: lint
lint: $(golint)
	$(golint) run ./...

.PHONY: clean
clean:
	rm -rf bin/

.PHONY: protos
protos: $(protoc-gen-go) $(protoc-gen-connect-go) $(buf)
	$(buf) lint protos
	$(buf) generate protos

.PHONY: version
version:
	@echo TAG_NAME=${TAG_NAME}
	@echo TAG_NAME_DEV=${TAG_NAME_DEV}
	@echo VERSION=${VERSION}
