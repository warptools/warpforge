package formulaexec

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"

	"github.com/warptools/warpforge/pkg/workspace"
	"github.com/warptools/warpforge/wfapi"
)

func memoizeRun(ws *workspace.Workspace, rr wfapi.RunRecord) wfapi.Error {
	// create the memo path, if it does not exist
	memoBasePath := ws.MemoBasePath()
	err := os.MkdirAll(ws.MemoBasePath(), 0755)
	if err != nil {
		return wfapi.ErrorIo("failed to create memo dir", memoBasePath, err)
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
		return wfapi.ErrorIo("failed to write memo file", memoPath, err)
	}

	return nil
}

func loadMemo(ws *workspace.Workspace, fid string) (*wfapi.RunRecord, wfapi.Error) {
	// if no workspace is provided, there can be no memos
	if ws == nil {
		return nil, nil
	}

	memoPath := ws.MemoPath(fid)[1:]
	fsys, _ := ws.Path()
	_, err := fs.Stat(fsys, memoPath)
	if errors.Is(err, fs.ErrNotExist) {
		// couldn't find a memo file, return nil to indicate there is no memo
		return nil, nil
	}
	if err != nil {
		// found memo file, but error reading, return error
		return nil, wfapi.ErrorIo("failed to stat memo file", memoPath, err)
	}

	// read the file
	f, err := fs.ReadFile(fsys, memoPath)
	if err != nil {
		return nil, wfapi.ErrorIo("failed to read memo file", memoPath, err)
	}

	memo := wfapi.RunRecord{}
	_, err = ipld.Unmarshal(f, json.Decode, &memo, wfapi.TypeSystem.TypeByName("RunRecord"))
	if err != nil {
		return nil, wfapi.ErrorSerialization(fmt.Sprintf("failed to deserialize memo file %q", memoPath), err)
	}

	return &memo, nil
}
