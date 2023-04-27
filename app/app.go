package wfapp

import (
	appbase "github.com/warptools/warpforge/app/base"
	_ "github.com/warptools/warpforge/app/catalog"
	_ "github.com/warptools/warpforge/app/check"
	_ "github.com/warptools/warpforge/app/enter"
	_ "github.com/warptools/warpforge/app/healthcheck"
	_ "github.com/warptools/warpforge/app/plan"
	_ "github.com/warptools/warpforge/app/quickstart"
	_ "github.com/warptools/warpforge/app/run"
	_ "github.com/warptools/warpforge/app/spark"
	_ "github.com/warptools/warpforge/app/status"
	_ "github.com/warptools/warpforge/app/ware"
	_ "github.com/warptools/warpforge/app/watch"
)

var App = appbase.App
