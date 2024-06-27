package server

import (
	"bufio"
	"context"
	"errors"
	"github.com/denzelpenzel/nyx/internal/db"
	"github.com/denzelpenzel/nyx/internal/logging"
	"github.com/denzelpenzel/nyx/internal/nyx"
	"github.com/denzelpenzel/nyx/internal/proto"
	"github.com/denzelpenzel/nyx/internal/proto/textprot"
	"go.uber.org/zap"
	"io"
	"log"
	"net"
	"time"
)

// ListenConst is a constructor function for listener implementations
type ListenConst func() (Listener, error)

// Listener is a type to accept and configure new connections
type Listener interface {
	Accept() (net.Conn, error)
	Configure(net.Conn) (net.Conn, error)
	GetAddr() string
}

type tcpListener struct {
	addr     net.Addr
	listener net.Listener
}

func (l *tcpListener) Accept() (net.Conn, error) {
	return l.listener.Accept()
}

func (l *tcpListener) Configure(conn net.Conn) (net.Conn, error) {
	tcpRemote := conn.(*net.TCPConn)

	if err := tcpRemote.SetKeepAlive(true); err != nil {
		return conn, err
	}

	if err := tcpRemote.SetKeepAlivePeriod(30 * time.Second); err != nil {
		return conn, err
	}

	return conn, nil
}

func (l *tcpListener) GetAddr() string {
	return l.addr.String()
}

func TCPListener(addr net.Addr) ListenConst {
	return func() (Listener, error) {
		listener, err := net.Listen("tcp", addr.String())
		if err != nil {
			return nil, err
		}
		return &tcpListener{
			addr:     addr,
			listener: listener,
		}, nil
	}
}

func shutdown(conn []io.Closer, err error) {
	if err != nil && !errors.Is(err, io.EOF) {
		log.Println("Error processing request, closing connection, error: ", err.Error())
	}
	for _, c := range conn {
		if c != nil {
			c.Close()
		}
	}
}

func ListenAndServe(ctx context.Context, l ListenConst, db db.DB, n nyx.NConst) {
	logger := logging.WithContext(ctx)
	ps := []proto.Components{textprot.Components}

	listener, err := l()
	if err != nil {
		panic(err)
	}

	logger.Info("Server successfully running", zap.String("addr", listener.GetAddr()))

	for {
		remote, err := listener.Accept()
		if err != nil {
			logger.Fatal("Failed to accept connection from remote", zap.Error(err))
			remote.Close()
			continue
		}

		remote, err = listener.Configure(remote)
		if err != nil {
			logger.Fatal("Failed to configure connection after accept", zap.Error(err))
		}

		if err != nil {
			logger.Fatal("Failed to create handler", zap.Error(err))
		}

		go func() {
			remoteReader := bufio.NewReader(remote)
			remoteWriter := bufio.NewWriter(remote)

			var reqParser proto.RequestParser
			var responder proto.Responder
			var matched bool

			peeker := proto.Peeker(remoteReader)

			for _, p := range ps {
				match, err := p.NewDisambiguator(peeker).CanParse()

				if err != nil {
					shutdown([]io.Closer{remote}, err)
					return
				}

				if match {
					reqParser = p.NewRequestParser(remoteReader)
					responder = p.NewResponder(remoteWriter)
					matched = true
				}
			}

			if !matched {
				p := ps[len(ps)-1]
				reqParser = p.NewRequestParser(remoteReader)
				responder = p.NewResponder(remoteWriter)
			}

			server := NewServer([]io.Closer{remote}, reqParser, n(db, responder))

			go server.Loop()
		}()
	}
}
