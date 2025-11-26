#!/bin/bash

rm -r ./exp/
mkdir ./exp/

for i in {0..3}; do
    go run cmd/consensusnode/main.go -shard_id=0 -node_id="${i}" &
done

go run cmd/supervisor/main.go -shard_id=0x7fffffff -node_id=0 &

wait
