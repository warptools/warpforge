package wfapi

import "github.com/ipld/go-ipld-prime/schema"

func init() {
	TypeSystem.Accumulate(schema.SpawnUnion("ApiOutput",
		[]schema.TypeName{
			"OutputString",
			"LogOutput",
			"RunRecord",
			"PlotResults",
		},
		schema.SpawnUnionRepresentationKeyed(map[string]schema.TypeName{
			"output":      "OutputString",
			"log":         "LogOutput",
			"runrecord":   "RunRecord",
			"plotresults": "PlotResults",
		})))
	TypeSystem.Accumulate(schema.SpawnString("OutputString"))
	TypeSystem.Accumulate(schema.SpawnString("LogString"))

	TypeSystem.Accumulate(schema.SpawnStruct("LogOutput", []schema.StructField{
		schema.SpawnStructField("msg", "LogString", false, false),
	}, schema.SpawnStructRepresentationMap(nil)))
}

type OutputString string
type LogString string

type LogOutput struct {
	Msg LogString
}

type ApiOutput struct {
	Output      *OutputString
	Log         *LogOutput
	RunRecord   *RunRecord
	PlotResults *PlotResults
}
