package main

import (
	"flag"
	"log"
	"strings"
	"time"

	"conncap/internal/protocol"
	"conncap/internal/server"
)

type stringListFlag []string

func (f *stringListFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringListFlag) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			*f = append(*f, part)
		}
	}
	return nil
}

func main() {
	host := flag.String("host", "::", "listen address")
	var bindIPs stringListFlag
	flag.Var(&bindIPs, "bind-ip", "bind listen IP, repeat or comma-separate for multiple IPs")
	tcpPort := flag.Int("tcp-port", protocol.DefaultTCPPort, "TCP port (0 to disable)")
	udpPort := flag.Int("udp-port", protocol.DefaultUDPPort, "UDP port (0 to disable)")
	statsPort := flag.Int("stats-port", protocol.DefaultStatsPort, "HTTP stats port (0 to disable)")
	interval := flag.Int("interval", 5, "stats log interval in seconds")
	v6Only := flag.Bool("v6-only", false, "listen IPv6 only (default dual-stack)")
	ipv6Prefix := flag.String("ipv6-prefix", "", "enable multi-IP IPv6 testing with this routed prefix, e.g. 2001:db8:1::/64")
	multiIPCount := flag.Int("multi-ip-count", server.DefaultMultiIPCount, "number of generated IPv6 multi-IP targets")
	flag.Parse()

	if *tcpPort == 0 && *udpPort == 0 {
		log.Fatal("at least one of -tcp-port or -udp-port must be > 0")
	}
	if *tcpPort < 0 || *udpPort < 0 || *statsPort < 0 {
		log.Fatal("ports must be >= 0")
	}
	if *interval <= 0 {
		log.Fatal("-interval must be > 0")
	}
	if *multiIPCount <= 0 {
		log.Fatal("-multi-ip-count must be > 0")
	}

	srv := server.New(*host, []string(bindIPs), *tcpPort, *udpPort, *statsPort, time.Duration(*interval)*time.Second, *v6Only, *ipv6Prefix, *multiIPCount)
	log.Printf("server starting: host=%s bind-ip=%s tcp=%d udp=%d stats=%d v6-only=%v ipv6-prefix=%s multi-ip-count=%d", *host, bindIPs.String(), *tcpPort, *udpPort, *statsPort, *v6Only, *ipv6Prefix, *multiIPCount)
	if err := srv.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
