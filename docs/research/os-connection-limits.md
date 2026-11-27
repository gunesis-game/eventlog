# Research: operating-system limits on HTTP connection pools

**Status:** Research note, 2026-11-27. It informs the file-system boundary transport decision (D-055); the decision and current design are owned by [architecture.md](../project/architecture.md). Figures come from the sources below and from platform defaults, which vary by version and configuration; verify them on the target system before relying on them operationally.

## Question

`internal/storage` reaches its file-system server over loopback HTTP (D-054). Concurrency grows with the number of Channels, because each Channel runs a Reader and a transaction processor that issue requests. What operating-system limits constrain how the client opens and reuses connections, and which transport avoids them without relying on deprecated mechanisms?

## Findings

### Ephemeral ports and TIME_WAIT

Each new loopback TCP connection takes an ephemeral port on the connecting side. The side that closes a connection holds its port in TIME_WAIT before it can be reused. Opening and closing connections faster than ports leave TIME_WAIT exhausts the range; `connect` then fails (`EADDRNOTAVAIL` on Linux) until ports free up.

| Platform | Default ephemeral range | TIME_WAIT | Approximate sustained ceiling on new connections |
|---|---|---|---|
| Windows | 49152–65535 (16,384 ports) | Controlled by `TcpTimedWaitDelay`; older guidance gives a 4-minute default, current systems are commonly 120 s | about 70–135 per second |
| Linux | 32768–60999 (about 28,232 ports) | 60 s by the kernel's TIME_WAIT constant (one source states 120 s) | about 235–470 per second |

Widening the range (`netsh int ipv4 set dynamicport`, `sysctl net.ipv4.ip_local_port_range`) or enabling `net.ipv4.tcp_tw_reuse` needs administrator rights, which an embedded library cannot assume. Long-lived connections avoid the problem instead of tuning around it.

### File descriptors and handles

A loopback connection between a client and a server in the same process costs two descriptors in that process. Since Go 1.19, Go programs on Unix raise the open-file soft limit to the hard limit; some distributions set the hard limit as low as 4,096, and systemd commonly sets the soft limit to 1,024. Windows handle limits are far higher and are not a practical constraint here.

### Go `net/http` connection pooling

- `Transport.MaxIdleConnsPerHost` defaults to 2 (`DefaultMaxIdleConnsPerHost`). It limits only how many idle connections are kept.
- When more requests are in flight than the idle limit allows, the transport opens extra connections and closes them after use; each closed connection leaves a port in TIME_WAIT on the client side.
- Only `Transport.MaxConnsPerHost` caps the total number of connections per host.
- A long-lived streaming response (for example, server-sent events) over HTTP/1.1 occupies one connection for its whole lifetime.

### Unencrypted HTTP/2, and what is deprecated

- **Deprecated:** the HTTP/1.1 `Upgrade: h2c` mechanism and the `HTTP2-Settings` header. RFC 9113 calls them deprecated (section 3.1) and obsolete, with the capability removed (section 11.2). Go's standard library does not support `Upgrade: h2c`.
- **Deprecated:** the `golang.org/x/net/http2/h2c` package, because unencrypted HTTP/2 moved into `net/http`.
- **Current:** starting HTTP/2 over cleartext TCP with prior knowledge (RFC 9113 section 3.3). Since Go 1.24, `net/http` supports it through `http.Protocols` and `SetUnencryptedHTTP2` on both `http.Server` and `http.Transport`, with no external dependency.
- HTTP/2 multiplexes concurrent requests as streams over one TCP connection; Go's server allows 250 concurrent streams per connection by default.
- The RFC wording above was confirmed through secondary sources (the Go issue tracker and implementer mailing lists), not a direct reading of the full RFC text.

### Unix domain sockets

Unix domain sockets use no ports, so TIME_WAIT does not apply. Go supports them on Linux and on Windows 10 version 1803 and later, but they bring a socket file to clean up after a crash, path-length limits, and Windows-specific caveats.

## Implications for eventlog

- Under the Stress profile (100 Channels), 200 or more concurrent requests would exceed a fixed HTTP/1.1 idle pool, so connections would churn; sustained churn above roughly 100 new connections per second could exhaust ephemeral ports on Windows.
- Capping total HTTP/1.1 connections avoids churn but makes the cap a throughput limit, and a server-sent-events stream would permanently hold one capped slot.
- HTTP/2 with prior knowledge over one connection removes churn regardless of Channel count, uses one port and two descriptors, uses no deprecated mechanism, and lets a future event stream share the same connection.

## Sources

- [Microsoft: Troubleshoot port exhaustion issues](https://learn.microsoft.com/windows/client-management/troubleshoot-tcpip-port-exhaust)
- [Microsoft: TCP/IP port exhaustion troubleshooting](https://learn.microsoft.com/en-us/troubleshoot/windows-client/networking/tcp-ip-port-exhaustion-troubleshooting)
- [Microsoft BizTalk: Settings that can be modified to improve network performance](https://learn.microsoft.com/biztalk/technical-guides/settings-that-can-be-modified-to-improve-network-performance)
- [Oracle: Tuning Windows parameters](https://docs.oracle.com/en/applications/enterprise-performance-management/11.2/hfmam/tuning_windows_parameters.html)
- [Cloudflare: How to stop running out of ephemeral ports and start to love long-lived connections](https://blog.cloudflare.com/how-to-stop-running-out-of-ephemeral-ports-and-start-to-love-long-lived-connections/)
- [netdev mailing list: local port range and TIME_WAIT exhaustion](https://lists.openwall.net/netdev/2007/03/06/49)
- [Go issue 13801: Transport ephemeral port exhaustion](https://golang.org/issue/13801)
- [golang-nuts: HTTP client connection reuse and growing TIME_WAIT](https://groups.google.com/g/golang-nuts/c/wliZf2_LUag)
- [Go source: net/http/transport.go](https://go.googlesource.com/go/+/HEAD/src/net/http/transport.go)
- [Go 1.19 release notes (RLIMIT_NOFILE)](https://go.dev/doc/go1.19)
- [Go issue 46279: raising RLIMIT_NOFILE](https://golang.org/issue/46279)
- [Go source: syscall/rlimit_test.go](https://golang.org/src/syscall/rlimit_test.go)
- [RFC 9113: HTTP/2](https://www.rfc-editor.org/rfc/rfc9113)
- [Go issue 63565: unencrypted HTTP/2 with prior knowledge](https://golang.org/issue/63565)
- [Go issue 67816: unencrypted HTTP/2 in net/http](https://github.com/golang/go/issues/67816)
- [Tomcat dev list: RFC 9113 h2c upgrade deprecation](https://www.mail-archive.com/dev@tomcat.apache.org/msg167251.html)
- [OpenJDK net-dev: HttpClient and RFC 9113](https://mail.openjdk.org/pipermail/net-dev/2025-January/025487.html)
- [pkg.go.dev: golang.org/x/net/http2/h2c (deprecated)](https://pkg.go.dev/golang.org/x/net/http2/h2c)
- [Go source: net/http/http.go (Protocols)](https://go.dev/src/net/http/http.go)
