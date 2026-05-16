# conncap

Language: [简体中文](README.md) | **English**

conncap is a TCP/UDP concurrent connection capacity tester for home broadband, routers, and ISP-side limits. It consists of a public server and a local client. The client creates many TCP or UDP sessions toward the server, while the server counts live sessions, peaks, and close reasons. conncap supports automatic IPv4/IPv6 detection and can also force IPv4 or IPv6 when needed.

## How It Works

```text
Home network client  ->  many TCP/UDP sessions  ->  public VPS server
```

- The server runs on a public VPS and listens for TCP/UDP test traffic.
- The client runs inside the network you want to test, such as a home router, OpenWrt device, PC, or downstream device.
- The client checks common local limits at startup, including open file limits, conntrack limits, and ephemeral port ranges. If these local limits may affect the result, conncap prints a bilingual warning.

## Download

Download prebuilt packages from GitHub Releases. Each package contains both the server and client binaries.

| OS | Arch | Package |
|---|---|---|
| Linux | x86_64 | `conncap-linux-amd64.tar.gz` |
| Linux | arm64 | `conncap-linux-arm64.tar.gz` |
| Linux | ARMv5+ | `conncap-linux-arm5.tar.gz` |
| Linux | MIPS big-endian | `conncap-linux-mips.tar.gz` |
| Linux | MIPS little-endian | `conncap-linux-mipsel.tar.gz` |
| macOS | Apple Silicon | `conncap-darwin-arm64.tar.gz` |
| Windows | x86_64 | `conncap-windows-amd64.zip` |

Common OpenWrt targets:

| OpenWrt target | Recommended package |
|---|---|
| x86_64 router | `conncap-linux-amd64.tar.gz` |
| aarch64/arm64 | `conncap-linux-arm64.tar.gz` |
| armv5/armv6/armv7 | `conncap-linux-arm5.tar.gz` |
| mipsel little-endian | `conncap-linux-mipsel.tar.gz` |
| mips big-endian | `conncap-linux-mips.tar.gz` |

## Quick Start

### 1. Start the server on a VPS

```bash
./conncap-server-linux-amd64
```

Default ports:

| Purpose | Default port |
|---|---|
| TCP test | `8888` |
| UDP test | `8888` |
| HTTP status API | Disabled by default; enable with `-stats-port 8889` |

Check server status:

```bash
./conncap-server-linux-amd64 -stats-port 8889
curl http://<vps_ip>:8889/status
```

To bind the server to specific listen IPs, pass `-bind-ip` more than once or use a comma-separated list:

```bash
./conncap-server-linux-amd64 -bind-ip 192.0.2.10 -bind-ip 2001:db8:1::1
./conncap-server-linux-amd64 -bind-ip 192.0.2.10,2001:db8:1::1
```

### 2. Run the client from the network being tested

TCP test, default mode:

```bash
./conncap-client-linux-amd64 -s <vps_ip>
```

UDP test:

```bash
./conncap-client-linux-amd64 -s <vps_ip> -u
```

TCP + UDP mixed test:

```bash
./conncap-client-linux-amd64 -s <vps_ip> -t -u
```

Force IPv6:

```bash
./conncap-client-linux-amd64 -s <vps_ipv6> -6
```

Force IPv4:

```bash
./conncap-client-linux-amd64 -s <vps_ipv4> -4
```

### IPv6 Multi-IP Test

If the VPS has a routed IPv6 prefix, the client can spread connections across multiple IPv6 destination addresses. This helps avoid the single-destination ephemeral port limit and makes higher connection counts observable. This mode is disabled by default and must be enabled manually on the server with `-ipv6-prefix`.

The correct syntax is `-bind-ip <control IPv6 address> -ipv6-prefix <test prefix>`. `-host` is kept only for compatibility with older versions and is not recommended for IPv6 multi-IP tests.

The server needs three things:

- The VPS provider has routed this IPv6 prefix to the current VPS.
- Linux has `net.ipv6.ip_nonlocal_bind=1` enabled.
- Linux has an IPv6 local route to `lo` covering the test prefix.

Temporarily enable non-local IPv6 binding:

```bash
sudo sysctl -w net.ipv6.ip_nonlocal_bind=1
```

Temporarily add the IPv6 local route:

```bash
sudo ip -6 route replace local <ipv6_prefix> dev lo
```

These two commands usually do not survive a reboot. For long-term use, write `net.ipv6.ip_nonlocal_bind=1` to `/etc/sysctl.conf` or `/etc/sysctl.d/*.conf`, and add the local route through your distribution's network configuration, startup script, or a systemd unit.

When starting the server, use `-bind-ip` to specify a specific reachable IPv6 control address on the VPS. Do not use the default `::` address for multi-IP mode:

```bash
./conncap-server-linux-amd64 -bind-ip <vps_ipv6> -ipv6-prefix 2001:db8:1::/64
```

For example, if the VPS interface has `fd12:3456:7890:100::a/56`, and the provider really routes `fd12:3456:7890:100::/56` to this VPS, you can use the whole `/56`:

```bash
sudo sysctl -w net.ipv6.ip_nonlocal_bind=1
sudo ip -6 route replace local fd12:3456:7890:100::/56 dev lo
./conncap-server-linux-amd64 -bind-ip fd12:3456:7890:100::a -ipv6-prefix fd12:3456:7890:100::/56
```

A more conservative option is to use only one `/64` from that allocation:

```bash
sudo ip -6 route replace local fd12:3456:7890:100::/64 dev lo
./conncap-server-linux-amd64 -bind-ip fd12:3456:7890:100::a -ipv6-prefix fd12:3456:7890:100::/64
```

Here `-bind-ip` is the reachable control address used by the client for the initial connection. `-ipv6-prefix` is the range used by the server to generate test destination IPs. The server skips the `-bind-ip` control address to avoid conflicts with dynamic listeners.

If your VPS panel shows `fd12:3456:7890:100::a/56` but you are not sure whether the whole `/56` is routed to this VPS, start with `/64`. Only expand both the local route and `-ipv6-prefix` to `/56` after confirming that the full prefix is usable.

Run the client against that control IPv6 address as usual. The server returns a generated IPv6 target list, and the client automatically round-robins across those targets:

```bash
./conncap-client-linux-amd64 -s <vps_ipv6> -6 -max 10000
```

By default the server generates `32` target IPv6 addresses. The control protocol uses a compact `IPRANGE` response, sending only the first IPv6 address and the count. The client generates the full target list locally, so you can increase the target count without transferring a long text list of IP addresses. For example, generate `1024` target IPs:

```bash
./conncap-server-linux-amd64 -bind-ip <vps_ipv6> -ipv6-prefix 2001:db8:1::/64 -multi-ip-count 1024
```

Note: larger `-multi-ip-count` values create more listening sockets on the server. With both TCP and UDP enabled, the listener socket count is roughly `multi-ip-count * 2 + control listeners`.

For UDP multi-IP tests, the server creates one listener for each generated IPv6 address so response packets use the same source address that the client targeted. UDP-only multi-IP mode still requires the server TCP port to be reachable, because the client uses a TCP control connection to fetch the target IP list.

## Client Options

```text
-s, -server string   Server address, required
-bind-ip string      Bind local IP, repeat or comma-separate
-t                  Enable TCP test; if neither -t nor -u is specified, TCP is enabled by default
-u                  Enable UDP test
-tcp-port int       Server TCP port, default 8888
-udp-port int       Server UDP port, default 8888
-max int            Target connection count, default 10000
-rate int           New connections per second, default 100, 0 means unlimited
-timeout duration   Connection timeout, default 5s
-keepalive duration Heartbeat interval, default 30s
-duration duration  Test duration, default 0 means unlimited
-4                  Force IPv4
-6                  Force IPv6
-stop-on-fail int   Stop after N seconds of peak plateau or N consecutive failures, default 100, 0 disables this behavior
```

## Server Options

```text
-host string       Legacy option: default IPv6 listen address, default ::; prefer -bind-ip for new configurations
-bind-ip string    Bind listen IP, repeat it or use comma-separated values; when set, only these IPs are listened on
-tcp-port int      TCP port, default 8888, 0 disables TCP
-udp-port int      UDP port, default 8888, 0 disables UDP
-stats-port int    HTTP status port, default 0 (disabled); pass a port such as 8889 to enable it
-interval int      Log interval in seconds, default 5
-v6-only           Listen on IPv6 only; default listens on both IPv4 and IPv6
-ipv6-prefix string Manually enable IPv6 multi-IP testing, for example 2001:db8:1::/64; requires at least one specific IPv6 -bind-ip
-multi-ip-count int Number of generated IPv6 multi-IP targets, default 32; can be raised to 256, 1024, etc.
```

## Reading Client Output

Example client output:

```text
attempt=3000 success=2995 failed=0 alive=2995 peak=2995
```

| Field | Meaning |
|---|---|
| `attempt` | Number of connection attempts made so far |
| `success` | Total successful connections over time |
| `failed` | Total failed connection attempts |
| `alive` | Connections currently alive |
| `peak` | Maximum simultaneously alive connections during the test |

For connection capacity testing, `peak` is usually the most important value. It represents the highest number of concurrent live sessions observed during the test.

## Reading Server Status

Example `/status` response:

```json
{
  "tcp_current": 1234,
  "tcp_peak": 2345,
  "udp_current": 567,
  "udp_peak": 890,
  "close_normal": 100,
  "close_timeout": 200,
  "close_reset": 0,
  "close_hello": 1,
  "close_other": 0,
  "uptime": "5m30s"
}
```

Close reason fields:

| Field | Meaning |
|---|---|
| `close_normal` | Peer closed the TCP connection normally |
| `close_timeout` | Read or heartbeat timeout |
| `close_reset` | TCP connection was reset with RST |
| `close_hello` | Connection failed during the HELLO handshake |
| `close_other` | Other close/error reasons |

## Before Testing

On Linux or OpenWrt, raise the file descriptor limit before high-count tests:

```bash
ulimit -n 1048576
```

The client checks these local limits at startup and prints a bilingual warning if a value may affect the result:

| Check | Possible impact |
|---|---|
| `Max open files` / `ulimit -n` | Maximum number of sockets the client process can open |
| `nf_conntrack_max` | Local conntrack table capacity |
| `ip_local_port_range` | Available ephemeral TCP source ports for one destination |

Start with a moderate connection rate:

```bash
./conncap-client-linux-amd64 -s <vps_ip> -rate 100
```

Increase `-rate` gradually if the network and devices remain stable.

## FAQ

### Why did I set `-max 10000`, but `peak` only reaches a few thousand?

`-max` is the target, not a guarantee. The real measured value is `peak`. If the client device, router, ISP, or server has a lower limit, `peak` will stop below the target.

### Why does `success` keep increasing while `alive` and `peak` stop growing?

`success` is cumulative. It can keep increasing if new connections are created while old connections are being closed or evicted. `alive` shows how many sessions are currently live, and `peak` shows the highest concurrent live count.

### Why can a downstream device reach a higher number than OpenWrt itself?

When the client runs directly on OpenWrt, every test session consumes OpenWrt's own file descriptors, socket memory, and local TCP/UDP state. When the client runs on a downstream PC, OpenWrt mostly forwards traffic, which uses a different resource profile. Check the startup warnings printed by the client first.

### Does IPv6 work automatically?

Yes. By default, the client auto-detects IPv4/IPv6. If the server address is IPv6, or a hostname has a reachable AAAA record, IPv6 will be used automatically. You can force IPv6 with `-6` or force IPv4 with `-4`.

### Does UDP mode create multiple sessions?

Yes. The UDP client creates a separate UDP socket for each session, giving each session a unique source port. The server tracks UDP sessions by source IP and source port.

### What should I do if the client warns about `open files`?

Raise the file descriptor limit before running the test:

```bash
ulimit -n 1048576
```

If the hard limit is also low, adjust your system or service manager configuration before testing. Otherwise, the result may reflect the local client limit rather than the real network limit.
