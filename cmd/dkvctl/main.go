package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"

	dkvv1 "distributed-kv/gen/dkv/v1"
	"distributed-kv/gen/dkv/v1/dkvv1connect"
	internaltls "distributed-kv/internal/tls"

	"connectrpc.com/connect"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v3"
	"golang.org/x/net/http2"
)

var (
	version string

	certFile      string
	keyFile       string
	trustedCAFile string
	endpoint      string
)

var (
	dkvClient              dkvv1connect.DkvAPIClient
	leaderDkvClient        dkvv1connect.DkvAPIClient
	membershipClient       dkvv1connect.MembershipAPIClient
	leaderMembershipClient dkvv1connect.MembershipAPIClient
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
	Before: func(c *cli.Context) (err error) {
		// TLS configuration
		var tlsConfig *tls.Config = nil
		if (certFile != "" && keyFile != "") || trustedCAFile != "" {
			tlsConfig, err = internaltls.SetupClientTLSConfig(certFile, keyFile, trustedCAFile)
			if err != nil {
				return err
			}
		}

		http := &http.Client{
			Transport: &http2.Transport{
				AllowHTTP: true,
				DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
					var d net.Dialer
					conn, err := d.DialContext(ctx, network, addr)
					if tlsConfig != nil {
						serverName, _, serr := net.SplitHostPort(addr)
						if serr != nil {
							serverName = addr
						}
						tlsConfig := tlsConfig.Clone()
						tlsConfig.ServerName = serverName
						return tls.Client(conn, tlsConfig), err
					}
					return conn, err
				},
			},
		}
		scheme := "http://"
		if tlsConfig != nil {
			scheme = "https://"
		}
		dkvClient = dkvv1connect.NewDkvAPIClient(http, scheme+endpoint, connect.WithGRPC())
		membershipClient = dkvv1connect.NewMembershipAPIClient(
			http,
			scheme+endpoint,
			connect.WithGRPC(),
		)
		leaderEndpoint := findEndpoint(c.Context)
		if leaderEndpoint == "" {
			leaderEndpoint = endpoint
		}
		leaderDkvClient = dkvv1connect.NewDkvAPIClient(
			http,
			scheme+leaderEndpoint,
			connect.WithGRPC(),
		)
		leaderMembershipClient = dkvv1connect.NewMembershipAPIClient(
			http,
			scheme+leaderEndpoint,
			connect.WithGRPC(),
		)
		return nil
	},
	// get, set, delete, member-join, member-leave, member-list
	Commands: []*cli.Command{
		{
			Name:      "get",
			Usage:     "Get the value of a key",
			ArgsUsage: "KEY",
			Action: func(c *cli.Context) error {
				ctx := c.Context
				key := c.Args().First()
				if key == "" {
					return cli.ShowCommandHelp(c, "get")
				}
				resp, err := dkvClient.Get(ctx, &connect.Request[dkvv1.GetRequest]{
					Msg: &dkvv1.GetRequest{
						Key: key,
					},
				})
				if err != nil {
					return err
				}
				fmt.Println(resp.Msg.GetValue())
				return nil
			},
		},
		{
			Name:      "set",
			Usage:     "Set the value of a key",
			ArgsUsage: "KEY VALUE",
			Action: func(c *cli.Context) error {
				ctx := c.Context
				key := c.Args().Get(0)
				value := c.Args().Get(1)
				if key == "" || value == "" {
					return cli.ShowCommandHelp(c, "set")
				}
				_, err := leaderDkvClient.Set(ctx, &connect.Request[dkvv1.SetRequest]{
					Msg: &dkvv1.SetRequest{
						Key:   key,
						Value: value,
					},
				})
				return err
			},
		},
		{
			Name:      "delete",
			Usage:     "Delete a key",
			ArgsUsage: "KEY",
			Action: func(c *cli.Context) error {
				ctx := c.Context
				key := c.Args().First()
				if key == "" {
					return cli.ShowCommandHelp(c, "delete")
				}
				_, err := leaderDkvClient.Delete(ctx, &connect.Request[dkvv1.DeleteRequest]{
					Msg: &dkvv1.DeleteRequest{
						Key: key,
					},
				})
				return err
			},
		},
		{
			Name:      "member-join",
			Usage:     "Join the cluster",
			ArgsUsage: "ID ADDRESS",
			Action: func(c *cli.Context) error {
				ctx := c.Context
				id := c.Args().Get(0)
				address := c.Args().Get(1)
				if id == "" || address == "" {
					return cli.ShowCommandHelp(c, "member-join")
				}
				_, err := leaderMembershipClient.JoinServer(
					ctx,
					&connect.Request[dkvv1.JoinServerRequest]{
						Msg: &dkvv1.JoinServerRequest{
							Id:      id,
							Address: address,
						},
					},
				)
				return err
			},
		},
		{
			Name:      "member-leave",
			Usage:     "Leave the cluster",
			ArgsUsage: "ID",
			Action: func(c *cli.Context) error {
				ctx := c.Context
				id := c.Args().First()
				if id == "" {
					return cli.ShowCommandHelp(c, "member-leave")
				}
				_, err := leaderMembershipClient.LeaveServer(
					ctx,
					&connect.Request[dkvv1.LeaveServerRequest]{
						Msg: &dkvv1.LeaveServerRequest{
							Id: id,
						},
					},
				)
				return err
			},
		},
		{
			Name:  "member-list",
			Usage: "List the cluster members",
			Action: func(c *cli.Context) error {
				ctx := c.Context
				resp, err := membershipClient.GetServers(
					ctx,
					&connect.Request[dkvv1.GetServersRequest]{
						Msg: &dkvv1.GetServersRequest{},
					},
				)
				if err != nil {
					return err
				}
				fmt.Println("ID\t| Raft Address\t| RPC Address\t| Leader")
				for _, server := range resp.Msg.GetServers() {
					fmt.Printf(
						"%s\t| %s\t| %s\t| %s\n",
						server.GetId(),
						server.GetRaftAddress(),
						server.GetRpcAddress(),
						strconv.FormatBool(server.GetIsLeader()),
					)
				}
				return nil
			},
		},
	},
}

func findEndpoint(ctx context.Context) (addr string) {
	servers, err := membershipClient.GetServers(ctx, &connect.Request[dkvv1.GetServersRequest]{
		Msg: &dkvv1.GetServersRequest{},
	})
	if err != nil {
		return ""
	}
	// No server? Use the RPC that was provided.
	if len(servers.Msg.GetServers()) == 0 {
		return ""
	}
	// Filter the server and only get the servers with RPC address
	advertisedServers := make([]*dkvv1.Server, len(servers.Msg.GetServers()))
	for _, server := range servers.Msg.GetServers() {
		if server.GetRpcAddress() != "" {
			advertisedServers = append(advertisedServers, server)
		}
	}
	// No advertised server? Use the RPC that was provided.
	if len(advertisedServers) == 0 {
		return ""
	}
	// Find the leader
	for _, server := range advertisedServers {
		// Request the first leader.
		if server.GetIsLeader() {
			return server.GetRpcAddress()
		}
	}

	// No leader? Request random server.
	idx := rand.Intn(len(advertisedServers))
	return advertisedServers[idx].GetRpcAddress()
}

func main() {
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load(".env")
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
