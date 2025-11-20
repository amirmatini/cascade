#!/bin/bash
set -e

PROXY_URL="http://localhost:3142"
TEST_URL="http://example.com"

if ! curl -s -x "$PROXY_URL" -I "$TEST_URL" > /dev/null 2>&1; then
    echo "Error: Cascade not running on $PROXY_URL"
    exit 1
fi

echo "Testing cache MISS..."
curl -s -x "$PROXY_URL" -I "$TEST_URL" | grep "X-Cache:" || echo "No cache header"

sleep 1

echo "Testing cache HIT..."
curl -s -x "$PROXY_URL" -I "$TEST_URL" | grep "X-Cache:" || echo "No cache header"

echo "Testing passthrough..."
curl -s -x "$PROXY_URL" -I "http://example.com/login" | head -1

echo "Done"

