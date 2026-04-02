#!/bin/bash
set -e

if [ ! -d "dist" ]; then
  echo "Error: dist directory not found"
  exit 1
fi

cd dist
for file in *; do
  if [[ "$file" == *.exe ]]; then
    zip "${file%.exe}.zip" "$file"
  else
    tar -czf "${file}.tar.gz" "$file"
  fi
done
cd ..
