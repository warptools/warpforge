mkdir -p $HOME/.warpforge/warehouse
rio mirror --source=file://plugins/alpine.tgz --target=ca+file://$HOME/.warpforge/warehouse tar:7KnxDuMATmnoQ1VFCLzxTT66Pdh7DfqJ5UGQSBniFFhD2yyAtfpuKNcdFf8Titvxxr

mkdir -p $HOME/.warpforge/bin
cp plugins/runc $HOME/.warpforge/bin
