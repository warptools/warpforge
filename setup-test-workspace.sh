mkdir -p .warpforge/warehouse

# add alpine image for testing
./plugins/rio mirror --source=https://dl-cdn.alpinelinux.org/alpine/v3.14/releases/x86_64/alpine-minirootfs-3.14.2-x86_64.tar.gz --target=ca+file://.warpforge/warehouse tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS

# add empty tarball for testing
#./plugins/rio mirror --source=file://plugins/test.tgz --target=ca+file://.warpforge/warehouse tar:7omHHaRUV3TcPYLk7VWTQgFSAWJa3HTRVwiZwESBy65w8rbrtVqdtZPg2nL1zXWPmR

mkdir -p .warpforge/bin
cp plugins/rio .warpforge/bin/
cp plugins/runc .warpforge/bin/
