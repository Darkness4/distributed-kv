package distributed

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/hashicorp/raft"
)

var _ raft.StreamLayer = (*TLSStreamLayer)(nil)

type TLSStreamLayer struct {
	net.Listener
	ServerTLSConfig *tls.Config
	ClientTLSConfig *tls.Config
}

func (s *TLSStreamLayer) Accept() (net.Conn, error) {
	conn, err := s.Listener.Accept()
	if err != nil {
		return nil, err
	}
	if s.ServerTLSConfig != nil {
		return tls.Server(conn, s.ServerTLSConfig), nil
	}
	return conn, nil
}

func (s *TLSStreamLayer) Addr() net.Addr {
	return s.Listener.Addr()
}

func (s *TLSStreamLayer) Close() error {
	return s.Listener.Close()
}

func (s *TLSStreamLayer) Dial(address raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}
	var conn, err = dialer.Dial("tcp", string(address))
	if err != nil {
		return nil, err
	}
	if s.ClientTLSConfig != nil {
		conn = tls.Client(conn, s.ClientTLSConfig)
	}
	return conn, err
}
