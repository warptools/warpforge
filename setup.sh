mkdir -p $HOME/.warpforge/warehouse
./plugins/rio mirror --source=file://plugins/alpine.tgz --target=ca+file://$HOME/.warpforge/warehouse tar:47Yg1Sdq21rPyDw9X9sCmRubQUADhFKe9G7qZCJRe61RhWPCxcQysCFzyCHffBKRjB

mkdir -p $HOME/.warpforge/bin
cp plugins/rio $HOME/.warpforge/bin/
cp plugins/runc $HOME/.warpforge/bin/
