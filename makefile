# Having these will allow CI scripts to build for many OS's and ARCH's
OS   := $(or ${OS},${OS},linux)
ARCH := $(or ${ARCH},${ARCH},amd64)

# Path to lint tool
GOLINTER ?= golangci-lint
GOFORMATTER ?= gofmt

# Determine binary file name
BIN_NAME := cpackget
PROG := build/$(BIN_NAME)
ifneq (,$(findstring windows,$(OS)))
    PROG=build/$(BIN_NAME).exe
endif

SOURCES := $(wildcard cmd/*.go) $(wildcard cmd/*/*.go)

all:
	@echo Pick one of:
	@echo $$ make $(PROG)
	@echo $$ make run
	@echo $$ make clean
	@echo $$ make config
	@echo $$ make release
	@echo
	@echo Build for different OS's and ARCH's by defining these variables. Ex:
	@echo $$ make OS=windows ARCH=amd64 build/$(BIN_NAME).exe  \# build for windows 64bits
	@echo $$ make OS=darwin  ARCH=amd64 build/$(BIN_NAME)      \# build for MacOS 64bits
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
	GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags "-X main.Version=`git describe 2>/dev/null || echo unknown`" -o $(PROG) ./cmd/

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
test:
	TESTING=1 go test $(ARGS) ./...

test-all: format-check coverage-check lint

coverage-report: 
	TESTING=1 go test ./... -coverprofile cover.out
	go tool cover -html=cover.out

coverage-check:
	TESTING=1 go test ./... $(ARGS) -coverprofile cover.out
	tail -n +2 cover.out | grep -v -e " 1$$" | grep -v main.go | tee coverage-check.out
	test ! -s coverage-check.out

release: test-all build/cpackget
	@./scripts/release

config:
	@echo "Configuring local environment"
	@go version 2>/dev/null || echo "Need Golang: https://golang.org/doc/install"
	@golangci-lint version 2>/dev/null || echo "Need GolangCi-Lint: https://golangci-lint.run/usage/install/#local-installation"

	# Install pre-commit hooks
	cp scripts/pre-commit .git/hooks/pre-commit
clean:
	rm -rf build/*
