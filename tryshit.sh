#!/bin/bash
set -euo pipefail

mkdir -p bin
go build -o bin/quack ./cmd/quack/*
GOPATH=$PWD/.gopath/ CGO_ENABLED=0 go build -o bin/smsh ./cmd/smsh/*
>&2 echo ":: go built"

mkdir -p rootfs/bin
cp -a bin/* rootfs/bin
>&2 echo ":: rootfs prepped"

rm config.json
./runc spec --rootless
#sed -i 's#"sh"#"/bin/quack", "in containment"#' config.json
sed -i 's#"sh"#"/bin/smsh"#' config.json
>&2 echo ":: runc config built"

./runc run --pid-file pid.pid yolo
>&2 echo ":: done"


