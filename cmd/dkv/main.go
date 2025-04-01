package main

import (
	"context"
	"crypto/tls"
	"distributed-kv/gen/dkv/v1/dkvv1connect"
	"distributed-kv/internal/api"
	"distributed-kv/internal/store/distributed"
	"distributed-kv/internal/store/persisted"
	internaltls "distributed-kv/internal/tls"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/raft"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v3"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var (
	version string

	name                string
	listenPeerAddress   string
	listenClientAddress string
	initialCluster      cli.StringSlice
	initialClusterState string
	advertiseNodes      cli.StringSlice

	peerCertFile      string
	peerKeyFile       string
	peerTrustedCAFile string

	certFile      string
	keyFile       string
	trustedCAFile string

	dataDir string
)

var app = &cli.App{
	Name:                 "dkv",
	Version:              version,
	Usage:                "Distributed Key-Value Store",
	Suggest:              true,
	EnableBashCompletion: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        "name",
			Usage:       "Unique name for this node",
			EnvVars:     []string{"DKV_NAME"},
			Destination: &name,
			Required:    true,
		},
		&cli.StringSliceFlag{
			Name:        "advertise-nodes",
			Usage:       "List of nodes to advertise",
			EnvVars:     []string{"DKV_ADVERTISE_NODES"},
			Destination: &advertiseNodes,
		},
		&cli.StringFlag{
			Name:        "listen-peer-address",
			Usage:       "Address to listen on for peer traffic",
			EnvVars:     []string{"DKV_LISTEN_PEER_ADDRESS"},
			Value:       ":2380",
			Destination: &listenPeerAddress,
		},
		&cli.StringFlag{
			Name:        "listen-client-address",
			Usage:       "Address listen on for client traffic",
			EnvVars:     []string{"DKV_LISTEN_CLIENT_ADDRESS"},
			Value:       ":3000",
			Destination: &listenClientAddress,
		},
		&cli.StringSliceFlag{
			Name:        "initial-cluster",
			Usage:       "Initial cluster configuration for bootstrapping",
			EnvVars:     []string{"DKV_INITIAL_CLUSTER"},
			Required:    true,
			Destination: &initialCluster,
		},
		&cli.StringFlag{
			Name:        "initial-cluster-state",
			Usage:       "Initial cluster state (new, existing)",
			EnvVars:     []string{"DKV_INITIAL_CLUSTER_STATE"},
			Required:    true,
			Destination: &initialClusterState,
		},
		&cli.StringFlag{
			Name:        "peer-cert-file",
			Usage:       "Path to the peer server TLS certificate file",
			EnvVars:     []string{"DKV_PEER_CERT_FILE"},
			Destination: &peerCertFile,
		},
		&cli.StringFlag{
			Name:        "peer-key-file",
			Usage:       "Path to the peer server TLS key file",
			EnvVars:     []string{"DKV_PEER_KEY_FILE"},
			Destination: &peerKeyFile,
		},
		&cli.StringFlag{
			Name:        "peer-trusted-ca-file",
			Usage:       "Path to the peer server TLS trusted CA certificate file",
			EnvVars:     []string{"DKV_PEER_TRUSTED_CA_FILE"},
			Destination: &peerTrustedCAFile,
		},
		&cli.StringFlag{
			Name:        "cert-file",
			Usage:       "Path to the client server TLS certificate file",
			EnvVars:     []string{"DKV_CERT_FILE"},
			Destination: &certFile,
		},
		&cli.StringFlag{
			Name:        "key-file",
			Usage:       "Path to the client server TLS key file",
			EnvVars:     []string{"DKV_KEY_FILE"},
			Destination: &keyFile,
		},
		&cli.StringFlag{
			Name:        "trusted-ca-file",
			Usage:       "Path to the client server TLS trusted CA certificate file",
			EnvVars:     []string{"DKV_TRUSTED_CA_FILE"},
			Destination: &trustedCAFile,
		},
		&cli.StringFlag{
			Name:        "data-dir",
			Usage:       "Path to the data directory",
			EnvVars:     []string{"DKV_DATA_DIR"},
			Value:       "data",
			Destination: &dataDir,
		},
	},
	Action: func(c *cli.Context) (err error) {
		ctx := c.Context
		// TLS configurations
		storeOpts := []distributed.StoreOption{}
		if peerCertFile != "" && peerKeyFile != "" {
			peerTLSConfig, err := internaltls.SetupServerTLSConfig(
				peerCertFile,
				peerKeyFile,
				peerTrustedCAFile,
			)
			if err != nil {
				return err
			}
			storeOpts = append(storeOpts, distributed.WithServerTLSConfig(peerTLSConfig))
		}

		if (peerCertFile != "" && peerKeyFile != "") || peerTrustedCAFile != "" {
			peerClientTLSConfig, err := internaltls.SetupClientTLSConfig(
				peerCertFile,
				peerKeyFile,
				peerTrustedCAFile,
			)
			if err != nil {
				return err
			}
			storeOpts = append(storeOpts, distributed.WithClientTLSConfig(peerClientTLSConfig))
		}

		var tlsConfig *tls.Config
		if certFile != "" && keyFile != "" {
			tlsConfig, err = internaltls.SetupServerTLSConfig(certFile, keyFile, trustedCAFile)
			if err != nil {
				return err
			}
		}

		// Store configuration
		store := persisted.New(dataDir)
		defer func() {
			_ = store.Close()
		}()
		dstore, err := bootstrapDStore(store, storeOpts)
		if err != nil {
			return err
		}
		defer func() {
			err := dstore.Shutdown()
			if err != nil {
				slog.Error("failed to shutdown store", "error", err)
			}
			slog.Warn("store shutdown")
		}()

		// Routes
		r := http.NewServeMux()
		r.Handle(dkvv1connect.NewDkvAPIHandler(&api.DkvAPIHandler{
			Store: dstore,
		}))

		nodes := make(map[raft.ServerID]string)
		for _, node := range advertiseNodes.Value() {
			id, addr, ok := strings.Cut(node, "=")
			if !ok {
				slog.Error("invalid initial cluster configuration", "node", node)
				continue
			}
			nodes[raft.ServerID(id)] = addr
		}
		r.Handle(dkvv1connect.NewMembershipAPIHandler(&api.MembershipAPIHandler{
			AdvertiseNodes: nodes,
			Store:          dstore,
		}))

		// Start the server
		l, err := net.Listen("tcp", listenClientAddress)
		if err != nil {
			return err
		}
		if tlsConfig != nil {
			l = tls.NewListener(l, tlsConfig)
		}
		slog.Info("server listening", "address", listenClientAddress)
		srv := &http.Server{
			BaseContext: func(_ net.Listener) context.Context { return ctx },
			Handler:     h2c.NewHandler(r, &http2.Server{}),
		}
		defer func() {
			_ = srv.Shutdown(ctx)
			_ = l.Close()
			slog.Warn("server shutdown")
		}()
		return srv.Serve(l)
	},
}

func bootstrapDStore(
	storer distributed.Storer,
	storeOpts []distributed.StoreOption,
) (dstore *distributed.Store, err error) {
	// Bootstrap
	nodes := initialCluster.Value()
	if len(nodes) == 0 {
		return nil, fmt.Errorf("invalid initial cluster configuration (no nodes): %s", nodes)
	}
	bootstrapNode, _, ok := strings.Cut(nodes[0], "=")
	if !ok {
		return nil, fmt.Errorf("invalid initial cluster configuration: %s", nodes)
	}
	advertizedPeers := make(map[raft.ServerID]raft.ServerAddress)
	for _, node := range nodes {
		id, addr, ok := strings.Cut(node, "=")
		if !ok {
			return nil, fmt.Errorf("invalid initial cluster configuration: %s", node)
		}
		advertizedPeers[raft.ServerID(id)] = raft.ServerAddress(addr)
	}

	dstore = distributed.NewStore(
		dataDir,
		listenPeerAddress,
		name,
		advertizedPeers[raft.ServerID(name)],
		storer,
		storeOpts...,
	)

	bootstrap := initialClusterState == "new" && bootstrapNode == name
	if err := dstore.Open(bootstrap); err != nil {
		return nil, err
	}
	// Periodically try to join the peers
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-dstore.ShutdownCh():
				slog.Error("stopped joining peers due to store shutdown")
				return
			case <-ticker.C:
				leaderAddr, leaderID := dstore.GetLeader()
				if leaderAddr == "" {
					slog.Error("no leader")
					continue
				}
				// Not leader
				if leaderID != raft.ServerID(name) {
					continue
				}
				members, err := dstore.GetServers()
				if err != nil {
					slog.Error("failed to get servers", "error", err)
					continue
				}
			peers:
				for id, addr := range advertizedPeers {
					// Ignore self
					if id == raft.ServerID(name) {
						continue
					}
					// Ignore if already member
					for _, member := range members {
						if member.ID == id {
							continue peers
						}
					}
					slog.Info("request peer to join", "id", id, "addr", addr)
					if err := dstore.Join(id, addr); err != nil {
						slog.Error("failed to join peer", "id", id, "addr", addr, "error", err)
					}
				}
			}
		}
	}()
	return dstore, nil
}

func main() {
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load(".env")
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
