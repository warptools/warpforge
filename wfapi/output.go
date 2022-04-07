package wfapi

type LogOutput struct {
	Msg string
}

type ApiOutput struct {
	Output      *string
	Log         *LogOutput
	RunRecord   *RunRecord
	PlotResults *PlotResults
}
