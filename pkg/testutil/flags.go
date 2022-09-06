package testutil

import "flag"

var FlagOffline = flag.Bool("testutil.offline", false, "Disable network usage in tests")
