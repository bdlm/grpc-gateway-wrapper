#!/bin/sh -x
protoc --version
for dir in "$@"; do
    go generate -x "$dir/..."
done
