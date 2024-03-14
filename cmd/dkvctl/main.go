package main

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"os"

	"distributed-kv/gen/dkv/v1/dkvv1connect"
	internaltls "distributed-kv/internal/tls"

	"connectrpc.com/connect"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"
	"golang.org/x/net/http2"
)

var (
	version string

	certFile      string
	keyFile       string
	trustedCAFile string
	endpoint      string
)

var app = &cli.App{
	Name:                 "dkvctl",
	Version:              version,
	Usage:                "Distributed Key-Value Store Client",
	Suggest:              true,
	EnableBashCompletion: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        "cert",
			Usage:       "Client certificate file",
			EnvVars:     []string{"DKVCTL_CERT"},
			Destination: &certFile,
		},
		&cli.StringFlag{
			Name:        "key",
			Usage:       "Client key file",
			EnvVars:     []string{"DKVCTL_KEY"},
			Destination: &keyFile,
		},
		&cli.StringFlag{
			Name:        "cacert",
			Usage:       "Trusted CA certificate file",
			EnvVars:     []string{"DKVCTL_CACERT"},
			Destination: &trustedCAFile,
		},
		&cli.StringFlag{
			Name:        "endpoint",
			Usage:       "Server endpoint",
			EnvVars:     []string{"DKVCTL_ENDPOINT"},
			Destination: &endpoint,
			Required:    true,
		},
	},
	Action: func(c *cli.Context) (err error) {
		ctx := c.Context

		// TLS configuration
		tlsConfig, err := internaltls.SetupClientTLSConfig(certFile, keyFile, trustedCAFile, "")
		if err != nil {
			return err
		}

		http := &http.Client{
			Transport: &http2.Transport{
				AllowHTTP: true,
				DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
					var d net.Dialer
					conn, err := d.DialContext(ctx, network, addr)
					if cfg != nil {
						return tls.Client(conn, cfg), err
					}
					return conn, err
				},
				TLSClientConfig: tlsConfig,
			},
		}
		client := dkvv1connect.NewDkvAPIClient(http, endpoint, connect.WithGRPC())
	},
}

func main() {
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load(".env")
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
