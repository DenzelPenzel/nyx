package network

import (
	"net"
	"time"
)

type Network struct {
	root net.Listener
}

func NewNetwork() *Network {
	return &Network{}
}

func (nt *Network) Open(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	nt.root = ln
	return nil
}

func (nt *Network) Dial(addr string, timeout time.Duration) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// Accept awaiting the next incoming connection
func (nt *Network) Accept() (net.Conn, error) {
	conn, err := nt.root.Accept()
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (nt *Network) Close() error {
	return nt.root.Close()
}

func (nt *Network) Addr() net.Addr {
	return nt.root.Addr()
}
