# conncap — Connection Capacity Tester

A TCP + UDP concurrent connection limit testing tool for home broadband. Tests the maximum number of concurrent connections your ISP/router allows. Supports IPv4 and IPv6 (dual-stack).

## Architecture

```
[Home Network Client]  ──massive TCP/UDP connections──>  [Public VPS Server]
```

- **Server**: Deploy on a VPS with a public IP. Listens on TCP/UDP ports, tracks live connection count and peaks, exposes an HTTP `/status` endpoint.
- **Client**: Run on your home network. Spawns massive concurrent connections to the server, reports success/failure/peak counts.

## Downloads

Prebuilt binaries in `build/`:

| OS | Arch | File |
|----|------|------|
| Linux | amd64 | `conncap-server-linux-amd64` / `conncap-client-linux-amd64` |
| Linux | arm64 | `conncap-server-linux-arm64` / `conncap-client-linux-arm64` |
| Linux | armv5+ | `conncap-server-linux-arm5` / `conncap-client-linux-arm5` |
| Linux | mips (BE, softfloat) | `conncap-server-linux-mips` / `conncap-client-linux-mips` |
| Linux | mipsel (LE, softfloat) | `conncap-server-linux-mipsel` / `conncap-client-linux-mipsel` |
| macOS | arm64 | `conncap-server-darwin-arm64` / `conncap-client-darwin-arm64` |
| Windows | amd64 | `conncap-server-windows-amd64.exe` / `conncap-client-windows-amd64.exe` |

All binaries are statically linked with zero runtime dependencies.

## Quick Start

### 1. Start the server (on your VPS)

```bash
./conncap-server-linux-amd64 --tcp-port 8888 --udp-port 8888 --stats-port 8889
```

Check status: `curl http://<vps_ip>:8889/status`

### 2. Run the client (on your home network)

```bash
# TCP only (default)
./conncap-client-linux-amd64 -s <vps_ip>

# UDP only
./conncap-client-linux-amd64 -s <vps_ip> -u

# TCP + UDP, IPv6
./conncap-client-linux-amd64 -s <vps_ipv6> -t -u -6
```

## Server Flags

```
Usage: server [flags]

  -host string       Listen address (default "::")
  -tcp-port int      TCP port, 0 to disable (default 8888)
  -udp-port int      UDP port, 0 to disable (default 8888)
  -stats-port int    HTTP status port, 0 to disable (default 8889)
  -interval int      Stats log interval in seconds (default 5)
  -v6-only           Listen IPv6 only (default dual-stack 0.0.0.0 + ::)
```

`/status` response:
```json
{"tcp_current":1234,"tcp_peak":2345,"udp_current":567,"udp_peak":890,
 "close_normal":100,"close_timeout":200,"close_reset":0,"close_hello":1,"close_other":0,
 "uptime":"5m30s"}
```

## Client Flags

```
Usage: client -s <address> [flags]

  -s, -server string  Server address (required)
  -t                  Enable TCP test (default when neither -t nor -u is set)
  -u                  Enable UDP test (default false)
  -tcp-port int       Server TCP port (default 8888)
  -udp-port int       Server UDP port (default 8888)
  -max int            Max connections to establish (default 10000)
  -rate int           Connections per second, 0 = unlimited (default 100)
  -timeout duration   Connection timeout (default 5s)
  -keepalive duration Heartbeat interval (default 30s)
  -duration duration  Test duration, 0 = unlimited (default 0)
  -4                  Force IPv4 only
  -6                  Force IPv6 only
  -stop-on-fail int   Stop after N sec peak plateau or N consecutive failures, 0 = never (default 100)
```

IPv4/IPv6 is auto-detected by default — no flag needed.

## Before Testing

1. **Raise file descriptor limit** (Linux):
   ```bash
   ulimit -n 1048576
   ```

2. **Start with a moderate rate** (e.g. `-rate 100`). Some ISPs throttle connections that ramp up too quickly.

3. **IPv6 is key**: If your home broadband is behind CGNAT (shared IPv4), the IPv4 connection limit is often very low (hundreds). IPv6 typically has no such limit.

4. **Port exhaustion**: A single client can open at most ~28,000 TCP connections due to ephemeral port range limits. For higher counts, run multiple client instances or bind multiple local IPs.

5. **Auto-stop**: The client stops automatically when peak stops growing for N seconds OR N consecutive connection failures occur. Default threshold is 100. Set `-stop-on-fail 0` to disable.

## Building from Source

```bash
go build -o conncap-server ./cmd/server
go build -o conncap-client ./cmd/client
```

## Protocol

Simple text-based protocol, delimited by `\n`:

| Direction | Message | Purpose |
|-----------|---------|---------|
| C → S | `HELLO` | TCP handshake |
| S → C | `OK` | Server acknowledgement |
| C → S | `PING` | Keepalive |
| S → C | `PONG` | Keepalive response |

UDP uses `REGISTER` instead of `HELLO` for initial contact.

---

# conncap — 连接容量测试工具

TCP + UDP 并发连接数测试工具，用于测试家庭宽带运营商/路由器允许的最大并发连接数。支持 IPv4 和 IPv6（双栈）。

## 架构

```
[家庭宽带客户端]  ──大量TCP/UDP连接──>  [公网VPS服务端]
```

- **服务端**：部署在具有公网 IP 的 VPS 上，监听 TCP/UDP 端口，实时统计连接数及峰值，提供 HTTP `/status` 状态接口。
- **客户端**：运行在家庭网络中，向服务端并发发起大量连接，报告成功/失败/峰值。

## 下载

预编译二进制文件在 `build/` 目录下：

| 系统 | 架构 | 文件 |
|------|------|------|
| Linux | amd64 | `conncap-server-linux-amd64` / `conncap-client-linux-amd64` |
| Linux | arm64 | `conncap-server-linux-arm64` / `conncap-client-linux-arm64` |
| Linux | armv5+ | `conncap-server-linux-arm5` / `conncap-client-linux-arm5` |
| Linux | mips (大端, softfloat) | `conncap-server-linux-mips` / `conncap-client-linux-mips` |
| Linux | mipsel (小端, softfloat) | `conncap-server-linux-mipsel` / `conncap-client-linux-mipsel` |
| macOS | arm64 | `conncap-server-darwin-arm64` / `conncap-client-darwin-arm64` |
| Windows | amd64 | `conncap-server-windows-amd64.exe` / `conncap-client-windows-amd64.exe` |

所有二进制均为静态编译，无任何运行时依赖。

## 快速开始

### 1. 启动服务端（在 VPS 上）

```bash
./conncap-server-linux-amd64 --tcp-port 8888 --udp-port 8888 --stats-port 8889
```

查看状态：`curl http://<vps_ip>:8889/status`

### 2. 运行客户端（在家庭网络中）

```bash
# 仅 TCP（默认）
./conncap-client-linux-amd64 -s <vps_ip>

# 仅 UDP
./conncap-client-linux-amd64 -s <vps_ip> -u

# TCP + UDP 混合，IPv6
./conncap-client-linux-amd64 -s <vps_ipv6> -t -u -6
```

## 服务端参数

```
用法: server [flags]

  -host string       监听地址（默认 "::"）
  -tcp-port int      TCP 端口，0 表示禁用（默认 8888）
  -udp-port int      UDP 端口，0 表示禁用（默认 8888）
  -stats-port int    HTTP 状态接口端口，0 表示禁用（默认 8889）
  -interval int      日志打印间隔秒数（默认 5）
  -v6-only           仅监听 IPv6（默认双栈 0.0.0.0 + ::）
```

`/status` 返回示例：
```json
{"tcp_current":1234,"tcp_peak":2345,"udp_current":567,"udp_peak":890,
 "close_normal":100,"close_timeout":200,"close_reset":0,"close_hello":1,"close_other":0,
 "uptime":"5m30s"}
```

## 客户端参数

```
用法: client -s <地址> [flags]

  -s, -server string  服务端地址（必填）
  -t                  启用 TCP 测试（未指定 -t/-u 时默认 TCP）
  -u                  启用 UDP 测试（默认 false）
  -tcp-port int       服务端 TCP 端口（默认 8888）
  -udp-port int       服务端 UDP 端口（默认 8888）
  -max int            最大连接目标数（默认 10000）
  -rate int           每秒新建连接数，0 不限制（默认 100）
  -timeout duration   连接超时（默认 5s）
  -keepalive duration 心跳间隔（默认 30s）
  -duration duration  测试时长，0 不限时（默认 0）
  -4                  强制仅 IPv4
  -6                  强制仅 IPv6
  -stop-on-fail int   peak 停滞 N 秒或连续 N 次失败后自动停止，0 永不停止（默认 100）
```

默认自动检测 IPv4/IPv6，无需手动指定。

## 测试前注意

1. **调大文件描述符限制**（Linux）：
   ```bash
   ulimit -n 1048576
   ```

2. **从较低的速率开始**（如 `-rate 100`），部分运营商检测到连接数过快增长会临时限速。

3. **IPv6 是重点**：如果家庭宽带处于 CGNAT（共享 IPv4）环境，IPv4 连接数通常很低（几百个），而 IPv6 一般无此限制——这是本工具的核心使用场景。

4. **端口耗尽**：单个客户端由于临时端口范围限制，最多约 28000 个 TCP 连接。如需更高数量，可运行多个客户端实例或绑定多个本地 IP。

5. **自动停止**：peak 停滞 N 秒不增长 或 连续 N 次建连失败，客户端自动停止。默认阈值 100。设 `-stop-on-fail 0` 禁用。

## 从源码编译

```bash
go build -o conncap-server ./cmd/server
go build -o conncap-client ./cmd/client
```

## 通信协议

简易文本协议，以 `\n` 分隔：

| 方向 | 消息 | 用途 |
|------|------|------|
| C → S | `HELLO` | TCP 客户端握手 |
| S → C | `OK` | 服务端确认 |
| C → S | `PING` | 心跳保活 |
| S → C | `PONG` | 心跳回复 |

UDP 初始接触使用 `REGISTER` 替代 `HELLO`。
