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

func (s *TLSStreamLayer) Dial(address raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.Dial("tcp", string(address))
	if s.ClientTLSConfig != nil {
		return tls.Client(conn, s.ClientTLSConfig), err
	}
	return conn, err
}
