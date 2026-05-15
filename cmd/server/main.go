package main

import (
	"flag"
	"log"
	"time"

	"conncap/internal/protocol"
	"conncap/internal/server"
)

func main() {
	host := flag.String("host", "::", "listen address")
	tcpPort := flag.Int("tcp-port", protocol.DefaultTCPPort, "TCP port (0 to disable)")
	udpPort := flag.Int("udp-port", protocol.DefaultUDPPort, "UDP port (0 to disable)")
	statsPort := flag.Int("stats-port", protocol.DefaultStatsPort, "HTTP stats port (0 to disable)")
	interval := flag.Int("interval", 5, "stats log interval in seconds")
	v6Only := flag.Bool("v6-only", false, "listen IPv6 only (default dual-stack)")
	flag.Parse()

	if *tcpPort == 0 && *udpPort == 0 {
		log.Fatal("at least one of -tcp-port or -udp-port must be > 0")
	}

	srv := server.New(*host, *tcpPort, *udpPort, *statsPort, time.Duration(*interval)*time.Second, *v6Only)
	log.Printf("server starting: host=%s tcp=%d udp=%d stats=%d v6-only=%v", *host, *tcpPort, *udpPort, *statsPort, *v6Only)
	if err := srv.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
