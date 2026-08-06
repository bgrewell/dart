# GO Parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GORUN=$(GOCMD) run

# Binary Name (edit if you don't want the default)
BINARY_NAME=$(shell basename $(CURDIR))

# Compiler flags
LD_FLAGS=-X 'main.version=$$(git describe --tags)' -X 'main.date=$$(date +"%Y.%m.%d_%H%M%S")' -X 'main.rev=$$(git rev-parse --short HEAD)' -X 'main.branch=$$(git rev-parse --abbrev-ref HEAD | tr -d '\040\011\012\015\n')'

# Tool Arguments
TAGS=json,yaml,xml

build: deps
	export GO111MODULE=on
	[ -d bin ] || mkdir bin
	GOOS=linux $(GOBUILD) -ldflags "$(LD_FLAGS)" -o bin/$(BINARY_NAME) -v ./cmd/dart

clean:
	$(GOCLEAN)
	rm -rf bin

# Downloads the dependencies pinned in go.mod. Upgrading dependencies is an
# explicit action (make deps-upgrade), not a side effect of every build —
# `go get -u` here made builds non-reproducible and broke them whenever an
# upstream module shipped a breaking change.
deps:
	export GOPRIVATE=github.com/bengrewell
	$(GOCMD) mod download

deps-upgrade:
	export GOPRIVATE=github.com/bengrewell
	$(GOGET) -u ./...
	$(GOCMD) mod tidy

install-tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go
	go get github.com/fatih/gomodifytags

run:
	$(GORUN) cmd/dart/main.go

tags:
	gomodifytags -file $(FILE) -all -add-tags $(TAGS) -w
