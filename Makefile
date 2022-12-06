VERSION=`git rev-parse --short HEAD`
#flags=-ldflags="-s -w -X main.version=${VERSION}"
OS := $(shell uname)
ifeq ($(OS),Darwin)
flags=-ldflags="-s -w -X main.version=${VERSION}"
else
flags=-ldflags="-s -w -X main.version=${VERSION} -extldflags -static"
endif

all: build

vet:
	go vet .

build:
	go clean; rm -rf pkg; CGO_ENABLED=0 go build -o wflow-dbs ${flags}

build_amd64: build_linux

build_darwin:
	go clean; rm -rf pkg wflow-dbs; GOOS=darwin CGO_ENABLED=0 go build -o wflow-dbs ${flags}

build_linux:
	go clean; rm -rf pkg wflow-dbs; GOOS=linux CGO_ENABLED=0 go build -o wflow-dbs ${flags}

build_power8:
	go clean; rm -rf pkg wflow-dbs; GOARCH=ppc64le GOOS=linux CGO_ENABLED=0 go build -o wflow-dbs ${flags}

build_arm64:
	go clean; rm -rf pkg wflow-dbs; GOARCH=arm64 GOOS=linux CGO_ENABLED=0 go build -o wflow-dbs ${flags}

build_windows:
	go clean; rm -rf pkg wflow-dbs; GOARCH=amd64 GOOS=windows CGO_ENABLED=0 go build -o wflow-dbs ${flags}

install:
	go install

clean:
	go clean; rm -rf pkg; rm wflow-dbs

test : test1

test1:
	go test -v -bench=.

release: clean build_amd64 build_arm64 build_windows build_power8 build_darwin
