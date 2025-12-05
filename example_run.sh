#!/bin/bash

rm -r ./exp/
mkdir ./exp/

set -ex

go mod download

go build ./...

for i in {0..3}; do
  for j in {0..3}; do
    go run cmd/consensusnode/main.go -shard_id="${j}" -node_id="${i}" &
  done
done

go run cmd/supervisor/main.go -shard_id=0x7fffffff -node_id=0 &

wait
