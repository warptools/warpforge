package formulaexec

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
)

func memoizeRun(ws *workspace.Workspace, rr wfapi.RunRecord) wfapi.Error {
	// create the memo path, if it does not exist
	memoBasePath := ws.MemoBasePath()
	err := os.MkdirAll(ws.MemoBasePath(), 0755)
	if err != nil {
		return wfapi.ErrorIo("failed to create memo dir", &memoBasePath, err)
	}

	// serialize the memo
	memoSerial, err := ipld.Marshal(json.Encode, &rr, wfapi.TypeSystem.TypeByName("RunRecord"))
	if err != nil {
		return wfapi.ErrorSerialization("failed to serialize memo", err)
	}

	// write the memo
	memoPath := ws.MemoPath(rr.FormulaID)
	err = os.WriteFile(memoPath, memoSerial, 0644)
	if err != nil {
		return wfapi.ErrorIo("failed to write memo file", &memoPath, err)
	}

	return nil
}

func loadMemo(ws *workspace.Workspace, fid string) (*wfapi.RunRecord, wfapi.Error) {
	memoPath := ws.MemoPath(fid)
	if _, err := os.Stat(memoPath); os.IsNotExist(err) {
		// couldn't find a memo file, return nil to indicate there is no memo
		return nil, nil
	} else if err != nil {
		// found memo file, but error reading, return error
		return nil, wfapi.ErrorIo("failed to stat memo file", &memoPath, err)
	}

	// read the file
	f, err := ioutil.ReadFile(memoPath)
	if err != nil {
		return nil, wfapi.ErrorIo("failed to read memo file", &memoPath, err)
	}

	memo := wfapi.RunRecord{}
	_, err = ipld.Unmarshal(f, json.Decode, &memo, wfapi.TypeSystem.TypeByName("RunRecord"))
	if err != nil {
		return nil, wfapi.ErrorSerialization(fmt.Sprintf("failed to deserialize memo file %q", memoPath), err)
	}

	return &memo, nil
}
