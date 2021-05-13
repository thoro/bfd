.PHONY: all test coverage coverage_html coverage_show proto

GIT_VERSION=$(shell git describe --tags --dirty)
GIT_COMMIT=$(shell git describe --always)
BUILD_DATE?=$(shell date +%Y-%m-%dT%H:%M:%S%z)

all: bfd bfdd

bfdd: 
	CGO_ENABLED=0 go build \
	-ldflags "-X main.version=$(GIT_VERSION) -X main.buildDate=$(BUILD_DATE) -X main.commit=$(GIT_COMMIT) " \
	-ldflags "-s -w" \
	-o bfdd github.com/Thoro/bfd/cmd/bfdd

bfd: 
	CGO_ENABLED=0 go build \
        -ldflags "-X main.version=$(GIT_VERSION) -X main.buildDate=$(BUILD_DATE) -X main.commit=$(GIT_COMMIT) " \
        -ldflags "-s -w" \
        -o bfd github.com/Thoro/bfd/cmd/bfd

fmt:
	go fmt \
		github.com/Thoro/bfd/pkg/packet/bfd \
		github.com/Thoro/bfd/pkg/server \
		github.com/Thoro/bfd/cmd/bfdd \
		github.com/Thoro/bfd/cmd/bfd


test:
	go test $(COVERAGE) \
		github.com/Thoro/bfd/pkg/packet/bfd \
		github.com/Thoro/bfd/pkg/server \
		github.com/Thoro/bfd/cmd/bfdd \
		github.com/Thoro/bfd/cmd/bfd

coverage: COVERAGE=--cover
coverage: test

coverage_show:
	go tool cover -html=coverage.out

coverage_html: COVERAGE=--coverprofile=coverage.out
coverage_html: test coverage_show

proto:
	protoc -I pkg/api --go_out=plugins=grpc:pkg/api pkg/api/api.proto


# PATH update
# export PATH=$PATH:/data/go/bin:/data/go/src/github.com/bin

#upx bfdd  -> 938 KB
