mkdir -p /tmp/u-root_build
pushd /tmp/u-root_build

echo "installing u-root"
go get github.com/u-root/u-root

echo "building u-root"
~/go/bin/u-root -base=/dev/null -build=bb -format=dir -o u-root

echo "doing rio pack"
rio pack --target=ca+file://$HOME/.warpforge/warehouse tar u-root

rm -rf /tmp/u-root_build

echo "done"
