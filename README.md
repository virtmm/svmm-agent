# svmm-agent

The SVMM agent runs on a compute node and talks to the SVMM server over **gRPC**.
The wire contract (CloudEvents carried on one bidirectional stream) lives in
[svmm-proto](https://github.com/propedeutica/svmm-proto).

**Status:** skeleton. The agent connects, sends a heartbeat on an interval and reconnects
with exponential backoff. Libvirt functionality is being moved over piece by piece from
the older `amqp-libvirt-bridge` agent (VM and network status, commands, storage). See
[Moving functionality](#moving-functionality).

## Build and run

Requires Go 1.24+.

```
make build
./bin/svmm-agent -server localhost:50051 -insecure
```

There is no transport security or authentication yet, so TLS is on by default and
`-insecure` (or `SVMM_INSECURE=true`) is needed to talk to a development server.

## Configuration

Priority: flags > environment > defaults.

| Flag | Environment | Default | |
|---|---|---|---|
| `-server` | `SVMM_SERVER_ADDR` | `localhost:50051` | gRPC server |
| `-agent-id` | `AGENT_ID` | `svma-<hostname>` | agent identity; sent as the CloudEvent `source` |
| `-heartbeat-interval` | `HEARTBEAT_INTERVAL` | `30s` | |
| `-insecure` | `SVMM_INSECURE` | `false` | disable TLS (development only) |

## Layout

```
cmd/svmm-agent/        main: wiring only
internal/config/       flags and environment
internal/agent/        stream session, heartbeats, reconnect
```

## Moving functionality

1. Define the message in `svmm-proto` (see its `docs/adding-a-message.md`) and tag a release.
2. Bump the dependency here: `go get github.com/propedeutica/svmm-proto@vX.Y.Z`.
3. Add the behaviour under `internal/` behind an interface (libvirt must be mockable) and
   handle the new event type in `internal/agent`.
4. Keep test coverage at or above 80%.

## Developing against a local svmm-proto

`go.mod` has a `replace` pointing at `../svmm-proto`. Remove it and pin a release tag
before publishing.

## License

Apache-2.0
