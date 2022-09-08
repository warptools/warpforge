package testutil

import "flag"

var FlagOffline = flag.Bool("offline", false, "Disable network usage in tests")
