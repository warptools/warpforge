UROOT=plugins/uroot.tar.gz
rio mirror --source=file://$UROOT --target=ca+file://$HOME/.warpforge/warehouse `rio scan tar --source=file://$UROOT`
