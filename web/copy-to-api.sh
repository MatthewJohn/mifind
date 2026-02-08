#!/bin/bash
# Copy React build files to Go embed directory

set -e

echo "Building React UI..."
npm run build

echo "Copying to Go embed directory..."
rm -rf ../internal/api/web/*
cp -r dist/* ../internal/api/web/

echo "✓ UI files copied successfully!"
echo "Now build the Go binary: go build ./cmd/mifind/main.go"
