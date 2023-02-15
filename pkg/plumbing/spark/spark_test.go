package spark

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"net"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	qt "github.com/frankban/quicktest"
	ipld "github.com/ipld/go-ipld-prime"
	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/plumbing/watch"
	"github.com/warptools/warpforge/pkg/testutil/nettest"
	"github.com/warptools/warpforge/pkg/workspaceapi"
	"github.com/warptools/warpforge/wfapi"
)

type server struct {
	recv workspaceapi.Rpc
	resp workspaceapi.Rpc
	l    net.Listener
}

var SchemaRpc = workspaceapi.TypeSystem.TypeByName("Rpc")

func (s *server) serve(t *testing.T) error {
	conn, err := s.l.Accept()
	if err != nil {
		return err
	}
	for {
		t.Logf("server receiving")
		_, err = ipld.UnmarshalStreaming(conn, watch.Decoder, &s.recv, SchemaRpc)
		if err != nil {
			return err
		}
		t.Logf("server recvd id: %q", s.recv.ID)
		t.Logf("server sending")
		err = ipld.MarshalStreaming(conn, watch.PrettyEncoder, &s.resp, workspaceapi.TypeSystem.TypeByName("Rpc"))
		if err != nil {
			return err
		}
	}
}

func NewLogBuffers(t *testing.T, ctx context.Context) (context.Context, func()) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	logger := logging.NewLogger(stdout, stderr, false, false, true)
	ctx = logger.WithContext(ctx)
	return ctx, func() {
		t.Log("---")
		t.Logf("flush stdout:\n%s", stdout.String())
		t.Logf("flush stderr:\n%s", stderr.String())
	}
}

func TestFailedDial(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	ctx, flush := NewLogBuffers(t, ctx)
	t.Cleanup(flush)
	pl := nettest.NewPipeListener(ctx)
	t.Cleanup(func() { pl.Close() })
	buf := &bytes.Buffer{}
	cfg := Config{
		WorkingDirectory: "/test/workspace",
		SearchPath:       "",
		Fsys: &fstest.MapFS{
			"test/workspace/module.wf":  &fstest.MapFile{Mode: 0755},
			"test/workspace/.warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
		},
		Dialer:       pl,
		OutputStream: buf,
		OutputMarkup: string(DefaultMarkup),
		OutputStyle:  string(DefaultStyle),
	}

	pl.Timeout = 0
	err := cfg.Run(ctx)
	qt.Assert(t, serum.Code(err), qt.Equals, ECodeSparkNoSocket)
	qt.Assert(t, wfapi.IsCode(err, wfapi.ECodeConnection), qt.IsTrue, qt.Commentf("expect code %q in error chain", wfapi.ECodeConnection))
}

func TestSparkNoWorkspace(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := Config{
		WorkingDirectory: "/test/workspace",
		SearchPath:       "subdir/nonexistent/dir",
		Fsys: &fstest.MapFS{
			"test/workspace/subdir/module.wf": &fstest.MapFile{Mode: 0755},
		},
		Dialer:       nil,
		OutputStream: buf,
		OutputMarkup: string(DefaultMarkup),
		OutputStyle:  string(DefaultStyle),
	}
	ctx := context.Background()
	err := cfg.Run(ctx)
	qt.Assert(t, err, qt.IsNotNil)
	qt.Assert(t, serum.Code(err), qt.Equals, ECodeSparkNoWorkspace)
}

func TestSpark(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	ctx, flush := NewLogBuffers(t, ctx)
	t.Cleanup(flush)
	pl := nettest.NewPipeListener(ctx)
	t.Cleanup(func() { pl.Close() })
	buf := &bytes.Buffer{}
	cfg := Config{
		WorkingDirectory: "/test/workspace",
		SearchPath:       "subdir/nonexistent/dir",
		Fsys: &fstest.MapFS{
			"test/workspace/subdir/module.wf": &fstest.MapFile{Mode: 0755},
			"test/workspace/.warpforge":       &fstest.MapFile{Mode: 0755 | fs.ModeDir},
		},
		Dialer:       pl,
		OutputStream: buf,
		OutputMarkup: string(DefaultMarkup),
		OutputStyle:  string(DefaultStyle),
	}

	expect := workspaceapi.ModuleStatusQuery{
		Path:          "subdir/module.wf",
		InterestLevel: workspaceapi.ModuleInterestLevel_Query,
	}
	resp := workspaceapi.ModuleStatusAnswer{
		Path:   "foobargrill",
		Status: workspaceapi.ModuleStatus_NoInfo,
	}
	srv := server{l: pl,
		resp: workspaceapi.Rpc{
			ID:   "1",
			Data: workspaceapi.RpcData{RpcResponse: &workspaceapi.RpcResponse{ModuleStatusAnswer: &resp}},
		},
	}
	srvCh := make(chan error)
	t.Cleanup(func() { close(srvCh) })
	// go func() { srvCh <- cfg.Run(ctx) }()
	go func() { srvCh <- srv.serve(t) }()

	err := cfg.Run(ctx)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("Run returned an error"))
	select {
	case err := <-srvCh:
		// an unexpected EOF is normal when the client closes the connection
		qt.Assert(t, errors.Is(err, io.ErrUnexpectedEOF), qt.IsTrue, qt.Commentf("Expect an error of %s", io.ErrUnexpectedEOF))
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
	qt.Assert(t, srv.recv.Data.RpcRequest.ModuleStatusQuery, qt.CmpEquals(), &expect)
	qt.Assert(t, strings.TrimSpace(buf.String()), qt.Equals, dasMap[Phase_NoPlan])
}
