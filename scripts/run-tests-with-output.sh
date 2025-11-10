#!/bin/bash
# Wrapper script to run tests and capture both stdout and stderr
# This ensures that test output is properly captured even when redirected

set -e

# Get the output file from the first argument, or use default
OUTPUT_FILE="${1:-build/test-output.txt}"

# Create build directory if it doesn't exist
mkdir -p "$(dirname "$OUTPUT_FILE")"

# Detect OS and ARCH like the makefile does
if command -v uname &> /dev/null; then
    DETECTED_OS=$(uname)
    DETECTED_ARCH=$(uname -m)
    
    # Determine OS
    if [[ "$DETECTED_OS" == *"indows"* ]] || [[ "$DETECTED_OS" == MINGW* ]] || [[ "$DETECTED_OS" == MSYS* ]]; then
        export GOOS=windows
    elif [[ "$DETECTED_OS" == "Darwin" ]]; then
        export GOOS=darwin
    else
        export GOOS=linux
    fi
    
    # Determine ARCH
    if [[ "$DETECTED_ARCH" == "x86_64" ]]; then
        export GOARCH=amd64
    elif [[ "$DETECTED_ARCH" == "aarch64" ]]; then
        export GOARCH=arm64
    else
        export GOARCH=amd64
    fi
fi

# Run tests and redirect both stdout and stderr to the output file
# Try make first, fall back to go test directly if make is not available
if command -v make &> /dev/null; then
    make test 2>&1 | tee "$OUTPUT_FILE"
    exit ${PIPESTATUS[0]}
else
    go test -v ./... -coverprofile ./cover.out 2>&1 | tee "$OUTPUT_FILE"
    exit ${PIPESTATUS[0]}
fi
