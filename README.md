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

只绑定指定监听 IP，可以重复传入 `-bind-ip`，也可以用逗号分隔：

```bash
./conncap-server-linux-amd64 -bind-ip 192.0.2.10 -bind-ip 2001:db8:1::1
./conncap-server-linux-amd64 -bind-ip 192.0.2.10,2001:db8:1::1
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

### IPv6 多 IP 测试

如果 VPS 拥有可路由的 IPv6 前缀，可以让客户端把连接分散到多个 IPv6 目标地址，用于绕开单目标临时端口数量限制，观察更高连接规模。该模式默认不启用，必须在服务端手动指定 `-ipv6-prefix`。

当前正确用法是 `-bind-ip <控制 IPv6 地址> -ipv6-prefix <测试前缀>`。`-host` 只是兼容旧版本的参数，不建议用于 IPv6 多 IP 测试。

服务端需要满足三个条件：

- VPS 提供商已经把这个 IPv6 前缀路由到当前 VPS。
- Linux 已开启 `net.ipv6.ip_nonlocal_bind=1`。
- Linux 有一条指向 `lo` 的 IPv6 local route，覆盖用于测试的前缀。

临时开启非本地 IPv6 绑定：

```bash
sudo sysctl -w net.ipv6.ip_nonlocal_bind=1
```

临时加入 IPv6 local route：

```bash
sudo ip -6 route replace local <ipv6_prefix> dev lo
```

这两个命令重启后通常会失效。长期使用时，需要按发行版把 `net.ipv6.ip_nonlocal_bind=1` 写入 `/etc/sysctl.conf` 或 `/etc/sysctl.d/*.conf`，并把 local route 写入系统网络配置、启动脚本或 systemd unit。

启动服务端时，需要用 `-bind-ip` 指定 VPS 上可连接的具体 IPv6 控制地址，不能使用默认的 `::`：

```bash
./conncap-server-linux-amd64 -bind-ip <vps_ipv6> -ipv6-prefix 2001:db8:1::/64
```

例如 VPS 网卡上有 `fd12:3456:7890:100::a/56`，并且提供商确实把 `fd12:3456:7890:100::/56` 路由到了这台 VPS，可以这样使用整个 `/56`：

```bash
sudo sysctl -w net.ipv6.ip_nonlocal_bind=1
sudo ip -6 route replace local fd12:3456:7890:100::/56 dev lo
./conncap-server-linux-amd64 -bind-ip fd12:3456:7890:100::a -ipv6-prefix fd12:3456:7890:100::/56
```

更保守的做法是只使用其中一个 `/64` 做测试：

```bash
sudo ip -6 route replace local fd12:3456:7890:100::/64 dev lo
./conncap-server-linux-amd64 -bind-ip fd12:3456:7890:100::a -ipv6-prefix fd12:3456:7890:100::/64
```

这里 `-bind-ip` 是客户端最初连接的控制地址，必须是 VPS 已经能从公网访问的具体 IPv6 地址。`-ipv6-prefix` 是服务端自动生成测试目标 IP 的范围。程序会跳过 `-bind-ip` 控制地址，避免和动态监听地址冲突。

如果你的 VPS 面板只显示 `fd12:3456:7890:100::a/56`，但不确定整个 `/56` 是否都路由给这台 VPS，优先用 `/64` 测试。只有确认整个 `/56` 可用时，再把 `ip route local` 和 `-ipv6-prefix` 都扩大到 `/56`。

客户端照常连接这个控制 IPv6 地址，服务端会返回生成的 IPv6 目标列表，客户端会自动轮询这些地址：

```bash
./conncap-client-linux-amd64 -s <vps_ipv6> -6 -max 10000
```

默认生成 `32` 个目标 IPv6。服务端使用紧凑的 `IPRANGE` 控制响应，只发送起始 IPv6 和数量，客户端会在本地生成完整目标列表，因此可以把目标数量调大而不会再传输很长的 IP 文本列表。例如生成 `1024` 个目标 IP：

```bash
./conncap-server-linux-amd64 -bind-ip <vps_ipv6> -ipv6-prefix 2001:db8:1::/64 -multi-ip-count 1024
```

注意：`-multi-ip-count` 越大，服务端启动时创建的 TCP/UDP 监听器越多。TCP 和 UDP 都启用时，监听 socket 数约为 `multi-ip-count * 2 + 控制监听器`。

UDP 多 IP 测试会在服务端为每个生成的 IPv6 地址创建独立监听器，这样回复包的源地址可以匹配客户端请求的目标地址。UDP-only 多 IP 模式仍需要服务端 TCP 端口可达，因为客户端使用 TCP 控制连接获取目标 IP 列表。

## 客户端参数

```text
-s, -server string   服务端地址，必填
-bind-ip string      绑定本地 IP，可重复传入或用逗号分隔
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
-host string       兼容旧参数：默认 IPv6 监听地址，默认 ::；新配置建议使用 -bind-ip
-bind-ip string    绑定监听 IP，可重复传入或用逗号分隔；传入后只监听这些 IP
-tcp-port int      TCP 端口，默认 8888，0 表示禁用
-udp-port int      UDP 端口，默认 8888，0 表示禁用
-stats-port int    HTTP 状态端口，默认 0（关闭），传入端口后启用，例如 8889
-interval int      日志输出间隔秒数，默认 5
-v6-only           仅监听 IPv6，默认同时监听 IPv4 和 IPv6
-ipv6-prefix string 手动启用 IPv6 多 IP 测试，例如 2001:db8:1::/64；需要至少一个具体 IPv6 -bind-ip
-multi-ip-count int IPv6 多 IP 目标数量，默认 32；可以提高到 256、1024 等
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
