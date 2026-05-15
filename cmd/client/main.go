package main

import (
	"flag"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"conncap/internal/client"
	"conncap/internal/protocol"
)

func main() {
	var serverAddr string
	flag.StringVar(&serverAddr, "s", "", "server address (required)")
	flag.StringVar(&serverAddr, "server", "", "server address (required)")
	tcpPort := flag.Int("tcp-port", protocol.DefaultTCPPort, "server TCP port")
	udpPort := flag.Int("udp-port", protocol.DefaultUDPPort, "server UDP port")
	useTCP := flag.Bool("t", false, "enable TCP test")
	useUDP := flag.Bool("u", false, "enable UDP test")
	max := flag.Int64("max", 10000, "max connections")
	rate := flag.Int("rate", 100, "connections per second (0 = unlimited)")
	timeout := flag.Duration("timeout", 5*time.Second, "connection timeout")
	keepalive := flag.Duration("keepalive", 30*time.Second, "keepalive interval")
	duration := flag.Duration("duration", 0, "test duration (0 = unlimited)")
	forceV4 := flag.Bool("4", false, "force IPv4")
	forceV6 := flag.Bool("6", false, "force IPv6")
	stopOnFail := flag.Int("stop-on-fail", 100, "stop after N sec peak stall or N consecutive failures")
	flag.Parse()

	if serverAddr == "" {
		log.Fatal("usage: client -s <address> [-t] [-u] [flags]")
	}

	if *forceV4 && *forceV6 {
		log.Fatal("cannot use both -4 and -6")
	}

	ipVersion := "auto"
	if *forceV4 {
		ipVersion = "4"
	}
	if *forceV6 {
		ipVersion = "6"
	}

	proto := "tcp"
	if *useTCP && *useUDP {
		proto = "both"
	} else if *useTCP {
		proto = "tcp"
	} else if *useUDP {
		proto = "udp"
	}

	if *max < 0 {
		log.Fatal("-max must be >= 0")
	}
	if *rate < 0 {
		log.Fatal("-rate must be >= 0")
	}
	if *timeout <= 0 {
		log.Fatal("-timeout must be > 0")
	}
	if *keepalive <= 0 {
		log.Fatal("-keepalive must be > 0")
	}
	if *duration < 0 {
		log.Fatal("-duration must be >= 0")
	}
	if *stopOnFail < 0 {
		log.Fatal("-stop-on-fail must be >= 0")
	}

	logSysLimits(*max, proto)

	cfg := client.Config{
		ServerAddr: serverAddr,
		TCPPort:    *tcpPort,
		UDPPort:    *udpPort,
		Protocol:   proto,
		Max:        *max,
		Rate:       *rate,
		Timeout:    *timeout,
		KeepAlive:  *keepalive,
		Duration:   *duration,
		IPVersion:  ipVersion,
		StopOnFail: *stopOnFail,
	}

	log.Printf("client starting: server=%s proto=%s max=%d rate=%d ipv=%s",
		serverAddr, proto, *max, *rate, ipVersion)

	c, err := client.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := c.Start(); err != nil {
		log.Fatal(err)
	}
}

func logSysLimits(targetMax int64, proto string) {
	if runtime.GOOS != "linux" {
		return
	}
	fdSoft := int64(-1)
	data, err := os.ReadFile("/proc/self/limits")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "Max open files") {
				log.Printf("sys: %s", strings.TrimSpace(line))
				fields := strings.Fields(line)
				if len(fields) >= 4 && fields[3] != "unlimited" {
					if v, err := strconv.ParseInt(fields[3], 10, 64); err == nil {
						fdSoft = v
					}
				}
				break
			}
		}
	}
	if fdSoft > 0 {
		// Each TCP/UDP session consumes one file descriptor in the client process.
		needed := targetMax + 128
		if targetMax == 0 {
			needed = 8192
		}
		if fdSoft < needed {
			log.Printf("WARN/警告: open files limit is low (soft=%d, target=%d). Results may be capped by the client OS instead of the network. Increase ulimit before testing. / open files 上限偏低（soft=%d，目标=%d），测试结果可能被客户端系统限制而非网络限制影响。建议测试前提高 ulimit。", fdSoft, targetMax, fdSoft, targetMax)
		}
	}

	conntrackMax := readProcInt("/proc/sys/net/netfilter/nf_conntrack_max")
	if conntrackMax > 0 && targetMax > 0 && conntrackMax < targetMax+1024 {
		log.Printf("WARN/警告: nf_conntrack_max=%d is close to target=%d. Local conntrack may affect results. / nf_conntrack_max=%d 接近目标连接数 %d，本机 conntrack 可能影响测试结果。", conntrackMax, targetMax, conntrackMax, targetMax)
	}

	portRange := readProcText("/proc/sys/net/ipv4/ip_local_port_range")
	if portRange != "" {
		parts := strings.Fields(portRange)
		if len(parts) >= 2 {
			lo, errLo := strconv.ParseInt(parts[0], 10, 64)
			hi, errHi := strconv.ParseInt(parts[1], 10, 64)
			if errLo == nil && errHi == nil && hi >= lo {
				ports := hi - lo + 1
				if targetMax > 0 && proto != "udp" && ports < targetMax+512 {
					log.Printf("WARN/警告: ephemeral port range has only %d ports, target=%d. TCP tests to one server may hit local port exhaustion. / 临时端口范围只有 %d 个端口，目标=%d，单服务端 TCP 测试可能先遇到本机端口耗尽。", ports, targetMax, ports, targetMax)
				}
			}
		}
	}

	for _, f := range []string{
		"/proc/sys/net/netfilter/nf_conntrack_max",
		"/proc/sys/net/ipv4/ip_local_port_range",
		"/proc/sys/fs/file-max",
	} {
		data, err := os.ReadFile(f)
		if err == nil {
			log.Printf("sys: %s = %s", f, data[:len(data)-1])
		}
	}
}

func readProcText(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readProcInt(path string) int64 {
	text := readProcText(path)
	if text == "" {
		return -1
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return -1
	}
	v, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return -1
	}
	return v
}
