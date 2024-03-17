# Distributed Key Value Store with Raft

- Raft as consensus algorithm.
- PebbleDB as storage for logs and stable storage for Raft.
- Protocol Buffers for serialization of commands for Raft.
- ConnectRPC (gRPC) for client-server communication.
- TCP over mutual TLS for server-server communication.
- Forked `hashicorp/raft` to add:
  - Support for command forwarding to the leader.
  - Proper advertized address for mutual TLS. (`hashicorp/raft` used to output the IP of the server instead of the domain name).

## How to compile and run:

This project uses a Makefile. Commands available are:

- `make all`: compiles the client and server.
- `make unit`: runs unit tests.
- `make integration`: runs integration tests.
- `make lint`: lint the code.
- `make fmt`: formats the code.
- `make protos`: compiles the protocol buffers into generated Go code.
- `make certs`: generates the certificates for the server, client and CA.
- `make clean`: cleans the project.

To run the server:

```bash
mkdir -p $(pwd)/dkv-0 $(pwd)/dkv-1 $(pwd)/dkv-2

dkv --name dkv-0 \
  --initial-cluster=dkv-0=localhost:2380,dkv-1=localhost:2381,dkv-2=localhost:2382 \
  --initial-cluster-state=new \
  --data-dir=$(pwd)/dkv-0 \
  --listen-peer-address=localhost:2380 \
  --listen-client-address=localhost:3000

dkv --name dkv-1 \
  --initial-cluster=dkv-0=localhost:2380,dkv-1=localhost:2381,dkv-2=localhost:2382 \
  --initial-cluster-state=new \
  --data-dir=$(pwd)/dkv-1 \
  --listen-peer-address=localhost:2381 \
  --listen-client-address=localhost:3001

dkv --name dkv-2 \
  --initial-cluster=dkv-0=localhost:2380,dkv-1=localhost:2381,dkv-2=localhost:2382 \
  --initial-cluster-state=new \
  --data-dir=$(pwd)/dkv-2 \
  --listen-peer-address=localhost:2382 \
  --listen-client-address=localhost:3002
```

To run the client:

```bash
dkvctl --endpoint=localhost:3000 set key value
dkvctl --endpoint=localhost:3000 get key
```

## Usages

**Server**

```shell
NAME:
   dkv - Distributed Key-Value Store

USAGE:
   dkv [global options] command [command options]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --name value                                         Unique name for this node [$DKV_NAME]
   --advertise-nodes value [ --advertise-nodes value ]  List of nodes to advertise [$DKV_ADVERTISE_NODES]
   --listen-peer-address value                          Address to listen on for peer traffic (default: ":2380") [$DKV_LISTEN_PEER_ADDRESS]
   --listen-client-address value                        Address listen on for client traffic (default: ":3000") [$DKV_LISTEN_CLIENT_ADDRESS]
   --initial-cluster value [ --initial-cluster value ]  Initial cluster configuration for bootstrapping [$DKV_INITIAL_CLUSTER]
   --initial-cluster-state value                        Initial cluster state (new, existing) [$DKV_INITIAL_CLUSTER_STATE]
   --peer-cert-file value                               Path to the peer server TLS certificate file [$DKV_PEER_CERT_FILE]
   --peer-key-file value                                Path to the peer server TLS key file [$DKV_PEER_KEY_FILE]
   --peer-trusted-ca-file value                         Path to the peer server TLS trusted CA certificate file [$DKV_PEER_TRUSTED_CA_FILE]
   --cert-file value                                    Path to the client server TLS certificate file [$DKV_CERT_FILE]
   --key-file value                                     Path to the client server TLS key file [$DKV_KEY_FILE]
   --trusted-ca-file value                              Path to the client server TLS trusted CA certificate file [$DKV_TRUSTED_CA_FILE]
   --data-dir value                                     Path to the data directory (default: "data") [$DKV_DATA_DIR]
   --help, -h                                           show help
   --version, -v                                        print the version
```

**Client**

```shell
NAME:
   dkvctl - Distributed Key-Value Store Client

USAGE:
   dkvctl [global options] command [command options]

COMMANDS:
   get           Get the value of a key
   set           Set the value of a key
   delete        Delete a key
   member-join   Join the cluster
   member-leave  Leave the cluster
   member-list   List the cluster members
   help, h       Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --cert value      Client certificate file [$DKVCTL_CERT]
   --key value       Client key file [$DKVCTL_KEY]
   --cacert value    Trusted CA certificate file [$DKVCTL_CACERT]
   --endpoint value  Server endpoint [$DKVCTL_ENDPOINT]
   --help, -h        show help
   --version, -v     print the version
```

## References

- [Blog Article](https://blog.mnguyen.fr/blog/2024-03-17-distributed-systems-in-go)
- [Raft](https://raft.github.io/)
- [Raft's Paper](https://raft.github.io/raft.pdf)
- [Distributed Services with Go - Traevis Jeffery](https://www.google.com/search?q=978-1680507607)

## License

This project is licensed under the Apache2 License - see the [LICENSE](LICENSE) file for details.
