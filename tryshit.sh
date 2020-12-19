#!/bin/bash
set -euo pipefail

mkdir -p bin
go build -o bin/quack ./cmd/quack/*
>&2 echo ":: go built"

mkdir -p rootfs
cp -a bin/ rootfs/
>&2 echo ":: rootfs prepped"

rm config.json
./runc spec --rootless
sed -i 's#"sh"#"/bin/quack", "in containment"#' config.json
>&2 echo ":: runc config built"

./runc run --pid-file pid.pid yolo
>&2 echo ":: done"


