#!/bin/bash
set -e

mkdir -p dist

for cmd in consensusnode supervisor; do
  for GOOS in linux windows darwin; do
    for GOARCH in amd64 arm64; do
      # Skip Windows arm64
      if [ "$GOOS" = "windows" ] && [ "$GOARCH" = "arm64" ]; then
        continue
      fi

      OUTPUT_NAME="blockemulator-${cmd}-${GOOS}-${GOARCH}"
      if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
      fi

      GOOS=$GOOS GOARCH=$GOARCH go build -o "dist/${OUTPUT_NAME}" "./cmd/${cmd}"
    done
  done
done
