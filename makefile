# Default to building for the host
OS ?= $(shell uname)

# Having this will allow CI scripts to build for many OS's and ARCH's
ARCH ?= $(shell uname -m)

# Retrieve version from git history
VERSION ?= $(shell git describe --tags 2>/dev/null || echo unknown)

# Path to lint tool
GOLINTER ?= golangci-lint
GOFORMATTER ?= gofmt

# Determine binary file name
BIN_NAME := cpackget
PROG := build/$(BIN_NAME)
ifneq (,$(findstring indows,$(OS)))
    PROG=build/$(BIN_NAME).exe
    OS=windows
else ifneq (,$(findstring Darwin,$(OS)))
    OS=darwin
else
    # Default to Linux
    OS=linux
endif
ifneq (,$(findstring x86_64,$(ARCH)))
	ARCH=amd64
else ifneq (,$(findstring aarch64,$(ARCH)))
    ARCH=arm64
else ifneq (,$(findstring unknown,$(ARCH)))
	# fallback
	ARCH=amd64
endif

SOURCES := $(wildcard cmd/*.go) $(wildcard cmd/*/*.go)

all:
	@echo Pick one of:
	@echo $$ make build
	@echo $$ make run
	@echo $$ make clean
	@echo $$ make config
	@echo $$ make release
	@echo
	@echo Build for different OS's and ARCH's by defining these variables. Ex:
	@echo $$ make OS=windows ARCH=amd64 build
	@echo $$ make OS=darwin ARCH=amd64 build
	@echo $$ make OS=linux ARCH=arm64 build
	@echo
	@echo Run tests
	@echo $$ make test ARGS="<test args>"
	@echo
	@echo Release a new version of $(BIN_NAME)
	@echo $$ make release
	@echo
	@echo Clean everything
	@echo $$ make clean
	@echo
	@echo Configure local environment
	@echo $$ make config
	@echo
	@echo Generate a report on code-coverage
	@echo $$ make coverage-report

$(PROG): $(SOURCES)
	@echo Building project
	GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags "-X main.version=$(VERSION)" -o $(PROG) ./cmd/

build: $(PROG)

run: $(PROG)
	@./$(PROG) $(ARGS) || true

lint:
	$(GOLINTER) run --config=.golangci.yml

format:
	$(GOFORMATTER) -s -w .

format-check:
	$(GOFORMATTER) -d . | tee format-check.out
	test ! -s format-check.out

.PHONY: test release config
test: $(SOURCES)
	cd cmd && GOOS=$(OS) GOARCH=$(ARCH) go test $(ARGS) ./... -coverprofile ../cover.out

test-all: format-check coverage-check lint

coverage-report: test
	go tool cover -html=cover.out

coverage-check: test
	@echo Checking if test coverage is atleast 84%
	test `go tool cover -func cover.out | tail -1 | awk '{print ($$3 + 0)*10}'` -ge 840

test-public-index:
	@./scripts/test-public-index

test-xmllint-localrepository: $(PROG)
	@./scripts/test-xmllint-localrepository

test-on-windows:
	@./scripts/test-on-windows

release: test-all $(PROG)
	@./scripts/release

config:
	@echo "Configuring local environment"
	@go version 2>/dev/null || echo "Need Golang: https://golang.org/doc/install"
	@golangci-lint version 2>/dev/null || echo "Need GolangCi-Lint: https://golangci-lint.run/usage/install/#local-installation"

	# Install pre-commit hooks
	cp scripts/pre-commit .git/hooks/pre-commit
clean:
	rm -rf build
