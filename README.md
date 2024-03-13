# Distributed Key Value Store with Raft

- Raft as consensus algorithm
- PebbleDB as storage for logs and stable storage for Raft
- Protocol Buffers for serialization of commands for Raft
- ConnectRPC (gRPC) for client-server communication
- TCP over mutual TLS for server-server communication

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
TODO
```

To run the client:

```bash
TODO
```

## References

- [Blog Article (TODO)]()
- [Raft](https://raft.github.io/)
- [Raft's Paper](https://raft.github.io/raft.pdf)
- [Distributed Services with Go - Traevis Jeffery](https://www.google.com/search?q=978-1680507607)

## License

This project is licensed under the Apache2 License - see the [LICENSE](LICENSE) file for details.
