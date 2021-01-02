#!/bin/bash
set -euo pipefail

git submodule update --init

export GOPATH=$PWD/.gopath/
export CGO_ENABLED=0
export GOBIN=$PWD/bin/

mkdir -p bin
go build -o bin/quack ./cmd/quack/*
go build -o bin/smsh ./cmd/smsh/*
>&2 echo ":: go built"

go install github.com/u-root/u-root
rm -rf /tmp/initramfs.linux_amd64.cpio # i can't figure out how to change this
bin/u-root -base=/dev/null -build=bb -format=dir --initcmd="" --defaultsh=""
>&2 echo ":: u-root built"

rm -rf rootfs
mkdir -p rootfs/bin
cp -a bin/{quack,smsh} rootfs/bin
cp -a /tmp/initramfs.linux_amd64.cpio/bbin/* rootfs/bin
>&2 echo ":: rootfs prepped"

rm -f config.json
./plugins/runc spec --rootless
#sed -i 's#"sh"#"/bin/quack", "in containment"#' config.json
sed -i 's#"sh"#"/bin/smsh", "--", "ls", "export ECHOME=heyy; export METOO=heyyyy", "interactive"#' config.json
>&2 echo ":: runc config built"

./plugins/runc run --pid-file pid.pid yolo
>&2 echo ":: done"


