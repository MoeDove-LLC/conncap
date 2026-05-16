# conncap

语言 / Language: **简体中文** | [English](README.en.md)

conncap 是一个用于测试家庭宽带、路由器或运营商侧并发连接能力的 TCP/UDP 连接数测试工具。它由服务端和客户端组成，支持 IPv4/IPv6 自动检测，也可以强制使用 IPv4 或 IPv6。

## 工作方式

```text
家庭网络客户端  ->  大量 TCP/UDP 会话  ->  公网 VPS 服务端
```

- 服务端运行在公网 VPS 上，负责接收连接并统计当前连接数、峰值和断开原因。
- 客户端运行在家庭网络、OpenWrt、PC 或其他待测环境中，负责主动发起大量连接。
- 客户端会自动检测本机常见限制，例如 `ulimit`、`conntrack`、临时端口范围，并在可能影响结果时输出中英文警告。

## 下载

请到 GitHub Releases 下载对应平台的压缩包。每个压缩包内都包含服务端和客户端两个程序。

| 系统 | 架构 | 文件 |
|---|---|---|
| Linux | x86_64 | `conncap-linux-amd64.tar.gz` |
| Linux | arm64 | `conncap-linux-arm64.tar.gz` |
| Linux | ARMv5+ | `conncap-linux-arm5.tar.gz` |
| Linux | MIPS 大端 | `conncap-linux-mips.tar.gz` |
| Linux | MIPS 小端 | `conncap-linux-mipsel.tar.gz` |
| macOS | Apple Silicon | `conncap-darwin-arm64.tar.gz` |
| Windows | x86_64 | `conncap-windows-amd64.zip` |

OpenWrt 常见架构：

| OpenWrt 架构 | 推荐文件 |
|---|---|
| x86_64 软路由 | `conncap-linux-amd64.tar.gz` |
| aarch64/arm64 | `conncap-linux-arm64.tar.gz` |
| armv5/armv6/armv7 | `conncap-linux-arm5.tar.gz` |
| mipsel 小端 | `conncap-linux-mipsel.tar.gz` |
| mips 大端 | `conncap-linux-mips.tar.gz` |

## 快速开始

### 1. 在 VPS 上启动服务端

```bash
./conncap-server-linux-amd64
```

默认监听：

| 用途 | 默认端口 |
|---|---|
| TCP 测试 | `8888` |
| UDP 测试 | `8888` |
| HTTP 状态接口 | 默认关闭，使用 `-stats-port 8889` 开启 |

查看服务端状态：

```bash
./conncap-server-linux-amd64 -stats-port 8889
curl http://<vps_ip>:8889/status
```

### 2. 在家庭网络中运行客户端

TCP 测试（默认）：

```bash
./conncap-client-linux-amd64 -s <vps_ip>
```

UDP 测试：

```bash
./conncap-client-linux-amd64 -s <vps_ip> -u
```

TCP + UDP 混合测试：

```bash
./conncap-client-linux-amd64 -s <vps_ip> -t -u
```

强制 IPv6：

```bash
./conncap-client-linux-amd64 -s <vps_ipv6> -6
```

强制 IPv4：

```bash
./conncap-client-linux-amd64 -s <vps_ipv4> -4
```

## 客户端参数

```text
-s, -server string   服务端地址，必填
-t                  启用 TCP 测试；如果没有指定 -t 或 -u，默认启用 TCP
-u                  启用 UDP 测试
-tcp-port int       服务端 TCP 端口，默认 8888
-udp-port int       服务端 UDP 端口，默认 8888
-max int            目标连接数，默认 10000
-rate int           每秒新建连接数，默认 100，0 表示不限速
-timeout duration   建连超时，默认 5s
-keepalive duration 心跳间隔，默认 30s
-duration duration  测试持续时间，默认 0，表示不限时
-4                  强制 IPv4
-6                  强制 IPv6
-stop-on-fail int   peak 停滞 N 秒或连续 N 次失败后停止，默认 100，0 表示禁用
```

## 服务端参数

```text
-host string       监听地址，默认 ::
-tcp-port int      TCP 端口，默认 8888，0 表示禁用
-udp-port int      UDP 端口，默认 8888，0 表示禁用
-stats-port int    HTTP 状态端口，默认 0（关闭），传入端口后启用，例如 8889
-interval int      日志输出间隔秒数，默认 5
-v6-only           仅监听 IPv6，默认同时监听 IPv4 和 IPv6
```

## 输出怎么看

客户端示例：

```text
attempt=3000 success=2995 failed=0 alive=2995 peak=2995
```

| 字段 | 含义 |
|---|---|
| `attempt` | 已尝试建立的连接数 |
| `success` | 累计成功建立的连接数 |
| `failed` | 累计失败连接数 |
| `alive` | 当前仍存活的连接数 |
| `peak` | 测试期间最大同时存活连接数 |

通常更应该关注 `peak`，它表示真正同时存在的最大连接数。

服务端状态接口示例：

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

断开原因说明：

| 字段 | 含义 |
|---|---|
| `close_normal` | 对端正常关闭 |
| `close_timeout` | 心跳或读超时 |
| `close_reset` | TCP RST 断开 |
| `close_hello` | 握手阶段失败 |
| `close_other` | 其他错误 |

## 测试前建议

Linux/OpenWrt 上建议先提高文件描述符上限：

```bash
ulimit -n 1048576
```

客户端启动时会自动检查以下限制，并在偏小时输出中英文警告：

| 检查项 | 可能影响 |
|---|---|
| `Max open files` / `ulimit -n` | 最大可打开 socket 数 |
| `nf_conntrack_max` | 本机连接跟踪表容量 |
| `ip_local_port_range` | 单目标 TCP 临时端口数量 |

建议从较低速率开始测试，例如：

```bash
./conncap-client-linux-amd64 -s <vps_ip> -rate 100
```

如果需要更快测试，再逐步提高 `-rate`。

## 常见问题

### 为什么我设置 `-max 10000`，但 `peak` 只有几千？

`-max` 是目标连接数，不代表一定能达到。真实结果看 `peak`。如果本机限制、路由器限制、运营商限制或服务端限制较低，`peak` 会停在较低数值。

### 为什么 `success` 很高，但 `alive` 和 `peak` 不涨？

这说明新连接能成功建立，但旧连接正在被关闭或淘汰。此时 `success` 是累计成功数，`alive` 才是当前存活数，`peak` 才是最大并发数。

### 为什么 OpenWrt 本机测试比下级设备低？

OpenWrt 本机运行客户端时，每条连接都会占用 OpenWrt 自己的 fd、socket 内存和本地 TCP 状态。下级设备运行客户端时，OpenWrt 主要负责转发，资源压力不同。请优先查看客户端启动时的限制提示。

### IPv6 会自动使用吗？

默认会自动检测。如果服务端地址是 IPv6，或域名有可用 AAAA 记录并且连通，客户端会自动使用 IPv6。也可以用 `-6` 强制 IPv6。

### UDP 测试是否真的会产生多个会话？

会。客户端会为每个 UDP 会话分配独立源端口，服务端按源 IP 和源端口统计 UDP 会话。
