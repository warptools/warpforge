package watch

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/wfapi"
)

// server stores the current status of the plot execution and responds to clients
type server struct {
	status   int
	listener net.Listener
}

// handle is expected to respond to client connections.
// This function should recover from panics and log errors before returning.
// It is expected that handle is run as a goroutine and that errors may not be handled.
//
// handle emits the current status of the watch command over the connection
func (s *server) handle(ctx context.Context, conn net.Conn) error {
	log := logging.Ctx(ctx)
	defer func() {
		r := recover()
		if r != nil {
			log.Info("", "socket handler panic: %s", r)
			return
		}
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// in lieu of doing anything complicated, shoving a status int down the pipe is sufficient
	// for the unix socket implementation we can now use netcat or similar for a status.
	// I.E. nc -U ./sock
	// OR socat - UNIX-CONNECT:./sock
	defer conn.Close()
	enc := json.NewEncoder(conn)
	err := enc.Encode(s.status)
	if err != nil {
		log.Info("", "socket handler: %s", err.Error())
		return err
	}
	return nil
}

// serve accepts and handles connections to the server.
// Serve should not return under normal circumstances, however if an error occurs then it will log that error and return it.
// Context will only be checked between accepted connections, canceling this function while it's blocking requires the server's listener to be closed.
func (s *server) serve(ctx context.Context) error {
	// It is expected that serve will be called as a goroutine and the returned error may not be handled.
	// Any errors should be logged before returning.
	log := logging.Ctx(ctx)
	if s.listener == nil {
		err := fmt.Errorf("did not call listen on server")
		log.Info("", err.Error())
		return err
	}
	for {
		conn, err := s.listener.Accept() // blocks, doesn't accept a context.
		if err != nil {
			log.Info("", "socket error on accept: %s", err.Error())
			return err
		}
		go s.handle(ctx, conn)
		select {
		case <-ctx.Done():
			log.Info("", "socket no longer accepting connections")
			return nil
		default:
		}
	}
}

// listen will create a unix socket on the given path
// listen should be called before "serve"
func (s *server) listen(ctx context.Context, sockPath string) (err error) {
	cfg := net.ListenConfig{}
	listener, err := cfg.Listen(ctx, "unix", sockPath)
	if err != nil {
		return wfapi.ErrorIo("could not create socket", sockPath, err)
	}
	s.listener = listener
	return nil
}
