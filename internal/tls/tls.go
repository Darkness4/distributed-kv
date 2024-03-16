package tls

import (
	"crypto/tls"
	"crypto/x509"
	"os"
)

func SetupServerTLSConfig(crt, key, ca string) (*tls.Config, error) {
	var cfg tls.Config
	if crt != "" && key != "" {
		certificate, err := tls.LoadX509KeyPair(crt, key)
		if err != nil {
			return nil, err
		}
		cfg.Certificates = []tls.Certificate{certificate}
	}
	if ca != "" {
		caCert, err := os.ReadFile(ca)
		if err != nil {
			return nil, err
		}
		cfg.ClientCAs = x509.NewCertPool()
		cfg.ClientCAs.AppendCertsFromPEM(caCert)
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return &cfg, nil
}

func SetupClientTLSConfig(crt, key, ca string) (*tls.Config, error) {
	var cfg tls.Config
	if crt != "" && key != "" {
		certificate, err := tls.LoadX509KeyPair(crt, key)
		if err != nil {
			return nil, err
		}
		cfg.Certificates = []tls.Certificate{certificate}
	}
	if ca != "" {
		caCert, err := os.ReadFile(ca)
		if err != nil {
			return nil, err
		}
		if cfg.RootCAs == nil {
			cas, err := x509.SystemCertPool()
			if err != nil {
				cfg.RootCAs = x509.NewCertPool()
			}
			cfg.RootCAs = cas
		}
		cfg.RootCAs.AppendCertsFromPEM(caCert)
	}
	return &cfg, nil
}
