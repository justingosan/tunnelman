#!/bin/bash

# E2E Test Runner for Tunnelman
# This script runs the end-to-end tests with proper setup and cleanup

set -e

echo "🧪 Starting Tunnelman E2E Tests..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go first."
    exit 1
fi

# Check if cloudflared is installed
if ! command -v cloudflared &> /dev/null; then
    echo "❌ cloudflared is not installed. Please install cloudflared first."
    exit 1
fi

# Check if config exists
CONFIG_FILE="$HOME/.tunnelman/config.json"
if [ ! -f "$CONFIG_FILE" ]; then
    echo "❌ Configuration file not found at $CONFIG_FILE"
    echo "Please run tunnelman first to create the configuration."
    exit 1
fi

# Check if API key is configured
if ! grep -q "cloudflare_api_key" "$CONFIG_FILE" || grep -q '"cloudflare_api_key": ""' "$CONFIG_FILE"; then
    echo "❌ Cloudflare API key not configured in $CONFIG_FILE"
    echo "Please configure your Cloudflare API key first."
    exit 1
fi

echo "✅ Prerequisites check passed"

# Set test timeout (default 10 minutes)
TIMEOUT=${E2E_TIMEOUT:-10m}

# Set verbosity
VERBOSE=${E2E_VERBOSE:-1}

# Test flags
TEST_FLAGS="-v -timeout=$TIMEOUT"

if [ "$VERBOSE" = "2" ]; then
    TEST_FLAGS="$TEST_FLAGS -count=1"
fi

# Run specific test if provided
if [ -n "$1" ]; then
    TEST_FLAGS="$TEST_FLAGS -run=$1"
    echo "🎯 Running specific test: $1"
else
    echo "🎯 Running all E2E tests"
fi

# Show test plan
echo "📋 Test Configuration:"
echo "   Timeout: $TIMEOUT"
echo "   Flags: $TEST_FLAGS"
echo ""

# Run the tests
echo "🚀 Executing tests..."
go test $TEST_FLAGS ./e2e_test.go

# Check test result
if [ $? -eq 0 ]; then
    echo ""
    echo "✅ All E2E tests passed!"
else
    echo ""
    echo "❌ Some E2E tests failed!"
    exit 1
fi

echo ""
echo "🎉 E2E test run completed successfully!"