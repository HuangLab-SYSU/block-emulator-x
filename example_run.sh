#!/bin/bash

SHARD_NUM=16
NODE_NUM=4

# Delete the old experiment directory.
rm -rf ./exp/
mkdir -p ./exp/

set -ex

# Download modules and pre-compile.
go mod download
go build ./...

# Start consensus nodes.
for ((i=0; i<SHARD_NUM; i++)); do
  for ((j=0; j<NODE_NUM; j++)); do
    go run cmd/consensusnode/main.go -shard_id="${i}" -node_id="${j}" &
  done
done

# Start the supervisor.
go run cmd/supervisor/main.go -shard_id=0x7fffffff -node_id=0 &

wait
