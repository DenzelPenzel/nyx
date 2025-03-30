package nyx

import (
	"net"
	"time"

	"github.com/hashicorp/raft"
)

type Listener interface {
	net.Listener
	// Dial - makes outgoing connections to other servers in the Raft cluster
	// When we connect to a server, we write the
	// RaftRPC byte to identify the connection type so we can multiplex Raft on the
	// same port as our Log gRPC requests
	Dial(addr string, timeout time.Duration) (net.Conn, error)
}

type RaftLayer struct {
	root Listener
}

func NewRaftLayer(ln Listener) *RaftLayer {
	return &RaftLayer{
		root: ln,
	}
}

func (l *RaftLayer) Dial(addr raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	return l.root.Dial(string(addr), timeout)
}

func (l *RaftLayer) Accept() (net.Conn, error) {
	return l.root.Accept()
}

func (l *RaftLayer) Close() error {
	return l.root.Close()
}

func (l *RaftLayer) Addr() net.Addr {
	return l.root.Addr()
}
