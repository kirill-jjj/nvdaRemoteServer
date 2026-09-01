# Introduction

This is a fork of the [Go NVDARemoteServer](https://github.com/tech10/nvdaRemoteServer) by tech10, reworked to be a near-exact, drop-in replacement for the original [Python NVDARemoteServer](https://github.com/jmdaweb/NVDARemoteServer) by jmdaweb.

The goal is simple: **the Python server's behavior, configuration and convenience, with the Go server's speed and memory footprint.**

What this means in practice:

- The same `option=value` configuration file (with `#` comments), the same default location (`/etc/NVDARemoteServer.conf`), the same command line flags, the same defaults. A configuration file written for the Python server works as is.
- The same daemon commands: `start`, `stop`, `restart`, `status`, `kill`, with the same PID-file behavior.
- Automatic generation of a self-signed TLS certificate if none is configured, so the server works out of the box — no certificate files, no entropy dances.
- Built-in Let's Encrypt / CertMagic integration: just pass `-domain your-domain.com` and certificates are obtained and renewed automatically.
- The full relay protocol is identical, and the server is dramatically faster than Python (see [Performance](#performance)).

# Goals

- **Drop-in Python compatibility.** Same config format, same flags, same defaults, same protocol. If you know how to run the Python server, you already know how to run this one.
- **Easy to run.** Daemon commands, PID files, systemd unit, Docker image, static builds — no runtime dependencies, no interpreter, no `pip install`.
- **Fast.** A pure-Go relay with no JSON parsing in the hot path (see [Performance](#performance)).
- **Reliable over flaky networks.** TCP keepalive, write deadlines and an application-level ping keep dead connections from hanging around forever.
- **Non-blocking relay.** Slow or disconnected clients never freeze the server — messages are dropped rather than blocking the channel.
- **Modern Go.** Built with Go 1.25+ idioms (ranging over integers, `testing.B.Loop`, `signal.NotifyContext`, `sync.RWMutex`, `strings.Cut`, `fmt.Errorf %w`, `log/slog` structured logging).

# Downloading / installation

## Binary releases

Pre-built binaries live on the [releases](https://github.com/kirill-jjj/nvdaRemoteServer/releases) page. The binaries are static where the target OS allows it, so they have no runtime dependencies.

For example, on a Linux x86_64 machine:

```console
$ wget https://github.com/kirill-jjj/nvdaRemoteServer/releases/latest/download/nvdaRemoteServer_linux_amd64.tar.gz
$ tar -axf ./nvdaRemoteServer_linux_amd64.tar.gz
$ cd ./nvdaRemoteServer_linux_amd64
$ sudo cp -a ./nvdaRemoteServer /usr/local/bin/
```

## Building from source

With Go 1.25 or newer installed:

```console
$ git clone https://github.com/kirill-jjj/nvdaRemoteServer
$ cd nvdaRemoteServer
$ go build -o nvdaRemoteServer .
```

Cross-compiling is supported for all platforms the daemon commands and TLS stack support:

```console
$ GOOS=linux  GOARCH=amd64 go build -o nvdaRemoteServer_linux_amd64 .
$ GOOS=windows GOARCH=amd64 go build -o nvdaRemoteServer_windows_amd64.exe .
$ GOOS=darwin GOARCH=arm64 go build -o nvdaRemoteServer_darwin_arm64 .
```

A fully static build (for musl-based systems) is available via the bundled script:

```console
$ ./build-static.sh
```

Note: the daemon commands (`start`/`stop`/`restart`/`status`/`kill`) are Unix-only. On Windows, they print a notice and exit; run the server directly instead.

## Docker

A `Dockerfile` is included. The image contains a certificate (`cert.pem`) so it can start without having to generate one:

```console
$ docker build -t nvdaremoteserver .
$ docker run --network host nvdaremoteserver /nvdaRemoteServer
```

# Quick start

The fastest way to get a server running:

```console
$ nvdaRemoteServer start -port 6837
Server started with pid 12345.

$ nvdaRemoteServer status
Server is running with pid 12345.
```

The server listens on port 6837 (IPv4 and IPv6), generates a self-signed TLS certificate in memory, and logs to the console plus the default log file (`/var/log/NVDARemoteServer/NVDARemoteServer.log` on Linux). The NVDA Remote addon can now connect to your machine's address on port 6837.

To stop it:

```console
$ nvdaRemoteServer stop
Stopping server with pid 12345...
Server stopped.
```

# Daemon commands

On Linux and other Unix-like systems the server can run as a background daemon, mirroring the Python server's commands. The daemon tracks the running instance via a PID file; the default is the server's `-pidfile` value (`/run/NVDARemoteServer/NVDARemoteServer.pid` on Linux). All arguments after the command are passed to the server itself, so the `-pidfile` parameter can be changed per command:

```console
$ nvdaRemoteServer start -port 6837 -pidfile /tmp/nvda.pid
Server started with pid 12345.

$ nvdaRemoteServer status -pidfile /tmp/nvda.pid
Server is running with pid 12345.

$ nvdaRemoteServer restart -pidfile /tmp/nvda.pid
Stopping server with pid 12345...
Server started with pid 12346.

$ nvdaRemoteServer stop -pidfile /tmp/nvda.pid
Stopping server with pid 12346...
Server stopped.

$ nvdaRemoteServer kill -pidfile /tmp/nvda.pid
Killing server with pid 12346...
Server killed.
```

- `stop` sends SIGTERM and waits for a graceful shutdown (existing connections are closed, the PID file is removed).
- `kill` sends SIGKILL; use it only if the server is frozen.
- `restart` stops the running instance (SIGTERM, then SIGKILL if it does not exit in time) and starts a fresh one.

A sample systemd unit is provided in `systemd/nvdaRemoteServer.service`.

## systemd

The repository includes `systemd/nvdaRemoteServer.service`. Edit the paths (and the `User=`/`Group=` values), then:

```console
$ sudo cp systemd/nvdaRemoteServer.service /etc/systemd/system/
$ sudo systemctl daemon-reload
$ sudo systemctl enable --now nvdaRemoteServer
```

# Configuration file

Like the Python server, this server reads a configuration file in the `option=value` format (UTF-8). The default location is `/etc/NVDARemoteServer.conf` on both Linux and macOS; the `-configfile` flag overrides it.

The format is identical to the Python server's — a configuration file written for the Python server works unchanged. A fully commented sample is provided in [`NVDARemoteServer.conf.example`](NVDARemoteServer.conf.example):

```console
# Comments start with a hash sign.
port=6837
port6=6837
interface=0.0.0.0
interface6=::
logfile=/var/log/NVDARemoteServer/NVDARemoteServer.log
pidfile=/run/NVDARemoteServer/NVDARemoteServer.pid
loglevel=2
keyfile=/usr/share/NVDARemoteServer/server.pem
certfile=/usr/share/NVDARemoteServer/server.pem
motd=Welcome to my server!
motd_force_display=0
includeTracebacks=0
allowedMessageLength=0
timeout=5.0
ping_time=300
```

Rules, matching the Python server exactly:

- Unknown options are ignored (so a Python-era config file works as is).
- Boolean options (`motd_force_display`, `includeTracebacks`) take `0` or `1`.
- Values are resolved with the priority **defaults < configuration file < command line**.
- If the configuration file can't be opened, the program prints `Can't open the configuration file, using default or commandline values` and continues.
- `includeTracebacks` is accepted for compatibility but has no effect: this Go server has no tracebacks.

If the configured certificate file does not exist, the server generates its own self-signed certificate in memory, so it works out of the box. To use a real certificate, point `certfile`/`keyfile` at your files (see [Let's Encrypt / CertMagic](#lets-encrypt--certmagic)).

# Command line reference

```
nvdaRemoteServer [-configfile /etc/NVDARemoteServer.conf] [-interface 0.0.0.0] [-interface6 ::]
                 [-port 6837] [-port6 6837] [-certfile /path/to/cert.pem] [-keyfile /path/to/key.pem]
                 [-pidfile /path/to/pid] [-logfile /path/to/log] [-loglevel 2] [-motd "message"]
                 [-motd_force_display=false] [-includeTracebacks=false] [-timeout 5.0]
                 [-ping_time 300] [-allowedMessageLength 0]
                 [-domain your-domain.com] [-acme_email you@example.com] [-acme_ca ""]
```

Brackets indicate optional parameters. Flags mirror the Python server by name and behavior; underscores can also be written with double dashes (`--ping_time=60`).

| Flag | Default | Description |
|---|---|---|
| `-configfile` | `/etc/NVDARemoteServer.conf` | Path to the configuration file (Python `option=value` format). |
| `-interface` | *(all)* | IPv4 interface to listen on. Does not affect IPv6. |
| `-interface6` | *(all)* | IPv6 interface to listen on. Does not affect IPv4. |
| `-port` | `6837` | TCP port for IPv4 connections (1–65535). |
| `-port6` | `6837` | TCP port for IPv6 connections; defaults to `-port`. |
| `-certfile` | *(empty)* | TLS certificate file. If missing/empty, a self-signed certificate is generated in memory. |
| `-keyfile` | *(empty)* | TLS key file. If missing/empty, a self-signed certificate is generated in memory. If both exist but are invalid, the server terminates. |
| `-pidfile` | `/run/NVDARemoteServer/NVDARemoteServer.pid` | PID file, created once the server has started and removed on shutdown. |
| `-logfile` | `/var/log/NVDARemoteServer/NVDARemoteServer.log` | Log file used in addition to the console. Falls back to console-only if the file can't be created. |
| `-loglevel` | `2` | Log verbosity, see below. |
| `-motd` | *(empty)* | Message of the day. |
| `-motd_force_display` | `false` | Force the MOTD to display even if unchanged since the last connect. |
| `-includeTracebacks` | `false` | Accepted for Python compatibility; no effect. |
| `-timeout` | `5.0` | Seconds a client may stay connected without negotiating TLS before being dropped. Values below 1.0 reset to the default. |
| `-ping_time` | `300` | Seconds between application-level pings to all clients. Values below 30 reset to the default. |
| `-allowedMessageLength` | `0` | Maximum length of incoming client messages in characters; longer messages get the client disconnected. `0` means no limit. |
| `-domain` | *(empty)* | Domain name for automatic TLS certificate management via Let's Encrypt / CertMagic. When set, certificates are obtained and renewed automatically. |
| `-acme_email` | *(empty)* | Email address for ACME registration (used with `-domain`). |
| `-acme_ca` | *(empty)* | Custom ACME CA URL (optional, for testing with staging servers). |

Logging levels (each level also logs everything below it):

- `-1` — disable logging except errors and panics.
- `0` — server start/stop and severe errors.
- `1` — client connections (ID and IP).
- `2` — channel joins/leaves, including channel passwords. Don't use in production.
- `3` — everything the program does at each stage; debugging.
- `4` — the raw protocol exchanged with clients; developers only.

### `version` and `buildinfo`

```console
$ nvdaRemoteServer version
development

$ nvdaRemoteServer buildinfo
This application was compiled with go1.25.0. It was compiled for the amd64 architecture and the linux operating system.
```

# TLS / certificates

## Automatic self-signed certificate

If no certificate is configured (or the configured file doesn't exist), the server generates a self-signed certificate in memory at startup. No files are written, nothing to manage — it simply works.

## Let's Encrypt / CertMagic

The server has built-in support for automatic certificate management via [CertMagic](https://github.com/caddyserver/certmagic). Just pass your domain:

```console
$ nvdaRemoteServer start -domain your-domain.com
```

Certificates are obtained automatically from Let's Encrypt and renewed in the background. No certbot, no hooks, no manual renewal.

For production, register an email for ACME notifications:

```console
$ nvdaRemoteServer start -domain your-domain.com -acme_email you@example.com
```

To use a custom ACME CA (e.g. for testing):

```console
$ nvdaRemoteServer start -domain your-domain.com -acme_ca https://acme-staging-v02.api.letsencrypt.org/directory
```

## Using your own certificate

Point `certfile` and `keyfile` at your PEM files (both may be the same file, as with the Python default):

```console
$ nvdaRemoteServer start -certfile /etc/ssl/nvda/server.pem -keyfile /etc/ssl/nvda/server.pem
```

## Legacy certbot hook

The repository also contains a post-renewal hook for [certbot](https://certbot.eff.org/), `NVDARemoteCertificate-letsencrypt`, which copies the renewed certificate into the file used by `certfile`/`keyfile`. This is not needed when using the built-in CertMagic integration.

# Protocol and behavior notes

The relay protocol is a faithful port of the Python server:

- Messages are relayed to **everyone except the sender** (the Python semantics — not the old Go server's master/slave routing).
- A client that is alone in a channel receives `nvda_not_connected`, so NVDA reports "NVDA Remote is not connected".
- The `origin` field is always added to relayed messages, exactly like Python (the old Go server only added it when `sendOrigin` was enabled, which it wasn't by default).
- Channel keys in the `lock_` form (e.g. `lock_nocontrol`) are understood and the MOTD explains the locked state, as in Python.
- The `nocontrol__password__controlme` / `lock_nocontrol__password__controlme` key scheme is supported: the first client to join with such a key sets the channel password.
- Authorization is not relayed (the Python server doesn't relay it either).

Conscious differences from the Python server, all invisible to clients:

- **JSON handling.** The Python server parses and re-serializes every message (`json.loads` + `json.dumps`); the Go server inserts the `origin` field with a byte-level insert and relays raw bytes, so the hot path does no JSON parsing at all. This is the main reason for the performance gap below.
- **Key generation.** 9-digit channel keys come from `crypto/rand` (the Python server uses `random.randint`, which is not cryptographically strong).
- **Reliability.** TCP keepalive (60 s), an 8-second write deadline and a 300 s application ping. The Python server has none of these; a half-dead client could hang around forever there.
- **Batching.** Outgoing messages are flushed with a smart batch (immediate when idle, ~5 ms when busy), so many small messages are coalesced into one TCP segment.
- **Non-blocking send.** If a client is too slow or disconnected, messages are dropped rather than blocking the entire channel. This prevents one slow client from freezing the server for everyone else.

# Performance

Numbers measured on localhost (TLS enabled, two clients, one relaying to the other):

| Scenario | This server | Python server |
|---|---|---|
| Relay throughput (5,000 messages) | 72,883 msg/s (~14 µs) | ~97 msg/s (~10 ms) |
| Relay throughput (20,000 messages) | 53,679 msg/s (~19 µs) | ~97 msg/s (~10 ms) |
| `origin` field insertion | 230 ns, 3 allocations | `json.loads`+`json.dumps` per message |

That's roughly **600× faster** than the Python server on relay throughput. The JSON microbenchmarks live in `server/jsonFuncs_test.go`:

```console
$ go test -bench . -benchmem -run '^$' ./server/
```

The old map-based JSON insertion (`Unmarshal` → map → `Marshal`) is kept there as `jsonAddOldMap` for comparison: 3,611 ns and 24 allocations vs. 230 ns and 3 for the byte-level insert.

# Bugs

Open an issue describing the bug and how to reproduce it. Be as detailed as you can — issues that can't be reproduced may be closed.

# Contributing

Fork the project and submit a pull request from a separate branch. Please:

- Test your contribution before submitting; make sure the program compiles (`go build ./...`), passes `go vet ./...`, and passes `golangci-lint run ./...`.
- Keep the protocol behavior unchanged unless the change is explicitly about it.
- For concurrency changes, run `go test -race ./...`.

# License

MIT, same as the original project.
