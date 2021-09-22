# This script does some jenky bootstrapping to set up a minimal workspace for tests.
# This allows both local testing, and for CI using Github Actions.
# Eventually, we'll have tools that take care of workpaces, which will replace this script.

# This script should be run from the root of the warpforge git repository.
# It will only work on x86_64 systems.

# create the workspace directory and warehouse directory
mkdir -p $HOME/.warpforge/warehouse

# add alpine image to the workspace's warehouse
./plugins/rio mirror --source=https://dl-cdn.alpinelinux.org/alpine/v3.14/releases/x86_64/alpine-minirootfs-3.14.2-x86_64.tar.gz --target=ca+file://$HOME/.warpforge/warehouse tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS

# create the workspace bin directory
mkdir -p $HOME/.warpforge/bin

# copy warpforge's binary dependancies
cp plugins/rio $HOME/.warpforge/bin/
cp plugins/runc $HOME/.warpforge/bin/
