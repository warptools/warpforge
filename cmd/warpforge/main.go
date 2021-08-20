package main

import (
	"io/ioutil"
	"log"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/warpfork/warpforge/pkg/formulaexec"
	"github.com/warpfork/warpforge/wfapi"
)

func main() {
	// read formula from pwd
	formula_file, err := ioutil.ReadFile("formula.json")
	if err != nil {
		log.Fatal(err)
	}
	frmAndCtx := wfapi.FormulaAndContext{}
	_, err = ipld.Unmarshal([]byte(formula_file), json.Decode, &frmAndCtx, wfapi.TypeSystem.TypeByName("FormulaAndContext"))
	if err != nil {
		log.Fatal(err)
	}

	err = formulaexec.Exec(frmAndCtx)
	if err != nil {
		log.Fatal(err)
	}
}
