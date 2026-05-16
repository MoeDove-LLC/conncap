package server

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type Stats struct {
	TCPCurrent      atomic.Int64
	TCPPeak         atomic.Int64
	UDPCurrent      atomic.Int64
	UDPPeak         atomic.Int64
	TCPCloseNormal  atomic.Int64
	TCPCloseTimeout atomic.Int64
	TCPCloseReset   atomic.Int64
	TCPCloseHello   atomic.Int64
	TCPCloseOther   atomic.Int64
}

type Server struct {
	Host         string
	BindIPs      []string
	TCPPort      int
	UDPPort      int
	StatsPort    int
	Interval     time.Duration
	IPv6Only     bool
	IPv6Prefix   string
	MultiIPCount int

	Stats          Stats
	StartTime      time.Time
	multiMu        sync.Mutex
	multiIPs       map[string]bool
	reservedBindIP map[string]bool
	multiIPList    []string
	multiIPReady   chan struct{}
}

func New(host string, bindIPs []string, tcpPort, udpPort, statsPort int, interval time.Duration, ipv6Only bool, ipv6Prefix string, multiIPCount int) *Server {
	return &Server{
		Host:           host,
		BindIPs:        bindIPs,
		TCPPort:        tcpPort,
		UDPPort:        udpPort,
		StatsPort:      statsPort,
		Interval:       interval,
		IPv6Only:       ipv6Only,
		IPv6Prefix:     ipv6Prefix,
		MultiIPCount:   multiIPCount,
		StartTime:      time.Now(),
		multiIPs:       make(map[string]bool),
		reservedBindIP: make(map[string]bool),
		multiIPReady:   make(chan struct{}),
	}
}

func (s *Server) Start() error {
	bindIPs, err := s.listenIPs()
	if err != nil {
		return err
	}
	s.BindIPs = bindIPs
	for _, ip := range bindIPs {
		s.reservedBindIP[ip] = true
	}

	if s.IPv6Prefix != "" {
		if !hasSpecificIPv6Bind(bindIPs) {
			return fmt.Errorf("-ipv6-prefix requires at least one specific IPv6 control listen address. Example: -bind-ip 2001:db8:1::1 -ipv6-prefix 2001:db8:1::/64 / 使用 -ipv6-prefix 时，至少需要一个具体 IPv6 控制监听地址。示例: -bind-ip 2001:db8:1::1 -ipv6-prefix 2001:db8:1::/64")
		}
		if err := checkIPv6NonlocalBind(); err != nil {
			return err
		}
		log.Printf("IPv6 multi-IP prefix enabled: %s", s.IPv6Prefix)
		s.initMultiIP()
	}

	if s.StatsPort > 0 {
		go s.startHTTPServer()
	}

	go s.logStats()
	activeListeners := 0

	if len(bindIPs) > 0 {
		for _, ip := range bindIPs {
			if s.TCPPort > 0 {
				if err := s.listenTCP(net.JoinHostPort(ip, fmt.Sprintf("%d", s.TCPPort)), networkForIP(ip, "tcp")); err != nil {
					return err
				}
				activeListeners++
			}
			if s.UDPPort > 0 {
				if err := s.listenUDP(net.JoinHostPort(ip, fmt.Sprintf("%d", s.UDPPort)), networkForIP(ip, "udp")); err != nil {
					return err
				}
				activeListeners++
			}
		}
	} else {
		if s.TCPPort > 0 {
			if s.IPv6Only {
				if err := s.listenTCP(fmt.Sprintf("[%s]:%d", s.Host, s.TCPPort), "tcp6"); err != nil {
					return err
				}
				activeListeners++
			} else {
				if err := s.listenTCP(fmt.Sprintf("0.0.0.0:%d", s.TCPPort), "tcp4"); err != nil {
					log.Printf("IPv4 listen failed (non-fatal): %v", err)
				} else {
					activeListeners++
				}
				if err := s.listenTCP(fmt.Sprintf("[%s]:%d", s.Host, s.TCPPort), "tcp6"); err != nil {
					log.Printf("IPv6 listen failed (non-fatal): %v", err)
				} else {
					activeListeners++
				}
			}
		}

		if s.UDPPort > 0 {
			if s.IPv6Only {
				if err := s.listenUDP(fmt.Sprintf("[%s]:%d", s.Host, s.UDPPort), "udp6"); err != nil {
					return err
				}
				activeListeners++
			} else {
				if err := s.listenUDP(fmt.Sprintf("0.0.0.0:%d", s.UDPPort), "udp4"); err != nil {
					log.Printf("UDP IPv4 listen failed (non-fatal): %v", err)
				} else {
					activeListeners++
				}
				if err := s.listenUDP(fmt.Sprintf("[%s]:%d", s.Host, s.UDPPort), "udp6"); err != nil {
					log.Printf("UDP IPv6 listen failed (non-fatal): %v", err)
				} else {
					activeListeners++
				}
			}
		}
	}
	if activeListeners == 0 {
		return fmt.Errorf("no active TCP/UDP listeners")
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("received signal %v, shutting down", sig)
	s.printFinalStats()
	return nil
}

func (s *Server) listenIPs() ([]string, error) {
	if len(s.BindIPs) > 0 {
		return normalizeBindIPs(s.BindIPs, s.IPv6Only)
	}
	if s.IPv6Prefix != "" {
		return normalizeBindIPs([]string{s.Host}, s.IPv6Only)
	}
	return nil, nil
}

func normalizeBindIPs(values []string, ipv6Only bool) ([]string, error) {
	seen := make(map[string]bool)
	ips := make([]string, 0, len(values))
	for _, value := range values {
		ip := net.ParseIP(strings.TrimSpace(value))
		if ip == nil {
			return nil, fmt.Errorf("invalid bind IP: %s / 无效监听 IP: %s", value, value)
		}
		if ipv6Only && ip.To4() != nil {
			return nil, fmt.Errorf("-v6-only cannot use IPv4 bind IP: %s / -v6-only 不能使用 IPv4 监听 IP: %s", value, value)
		}
		normalized := ip.String()
		if !seen[normalized] {
			seen[normalized] = true
			ips = append(ips, normalized)
		}
	}
	return ips, nil
}

func networkForIP(ip, proto string) string {
	parsed := net.ParseIP(ip)
	if parsed != nil && parsed.To4() != nil {
		return proto + "4"
	}
	return proto + "6"
}

func hasSpecificIPv6Bind(ips []string) bool {
	for _, ip := range ips {
		parsed := net.ParseIP(ip)
		if parsed != nil && parsed.To4() == nil && !parsed.IsUnspecified() {
			return true
		}
	}
	return false
}

func (s *Server) listenTCP(address, network string) error {
	return s.listenTCPWithLog(address, network, true)
}

func (s *Server) listenTCPWithLog(address, network string, shouldLog bool) error {
	tcpAddr, err := net.ResolveTCPAddr(network, address)
	if err != nil {
		return fmt.Errorf("resolve TCP %s: %w", network, err)
	}
	ln, err := net.ListenTCP(network, tcpAddr)
	if err != nil {
		return fmt.Errorf("listen TCP %s: %w", network, err)
	}
	if shouldLog {
		log.Printf("TCP listening on %s (%s)", address, network)
	}
	go s.acceptTCP(ln)
	return nil
}

func (s *Server) listenUDP(address, network string) error {
	return s.listenUDPWithLog(address, network, true)
}

func (s *Server) listenUDPWithLog(address, network string, shouldLog bool) error {
	udpAddr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return fmt.Errorf("resolve UDP %s: %w", network, err)
	}
	conn, err := net.ListenUDP(network, udpAddr)
	if err != nil {
		return fmt.Errorf("listen UDP %s: %w", network, err)
	}
	if shouldLog {
		log.Printf("UDP listening on %s (%s)", address, network)
	}
	go handleUDP(conn, &s.Stats)
	return nil
}

func checkIPv6NonlocalBind() error {
	data, err := os.ReadFile("/proc/sys/net/ipv6/ip_nonlocal_bind")
	if err != nil {
		return fmt.Errorf("IPv6 multi-IP requires Linux net.ipv6.ip_nonlocal_bind=1; cannot read /proc/sys/net/ipv6/ip_nonlocal_bind: %w / IPv6 多 IP 需要 Linux net.ipv6.ip_nonlocal_bind=1，但无法读取该配置: %w", err, err)
	}
	if strings.TrimSpace(string(data)) != "1" {
		return fmt.Errorf("IPv6 multi-IP requires net.ipv6.ip_nonlocal_bind=1. Enable it with: sysctl -w net.ipv6.ip_nonlocal_bind=1 / IPv6 多 IP 测试需要开启 net.ipv6.ip_nonlocal_bind=1，请先执行: sysctl -w net.ipv6.ip_nonlocal_bind=1")
	}
	return nil
}

func (s *Server) logStats() {
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for range ticker.C {
		tcpCur := s.Stats.TCPCurrent.Load()
		tcpPeak := s.Stats.TCPPeak.Load()
		udpCur := s.Stats.UDPCurrent.Load()
		udpPeak := s.Stats.UDPPeak.Load()
		normal := s.Stats.TCPCloseNormal.Load()
		tout := s.Stats.TCPCloseTimeout.Load()
		rst := s.Stats.TCPCloseReset.Load()
		hello := s.Stats.TCPCloseHello.Load()
		other := s.Stats.TCPCloseOther.Load()
		log.Printf("TCP: cur=%d peak=%d | UDP: cur=%d peak=%d | close: normal=%d timeout=%d reset=%d hello=%d other=%d",
			tcpCur, tcpPeak, udpCur, udpPeak, normal, tout, rst, hello, other)
	}
}

func (s *Server) printFinalStats() {
	tcpCur := s.Stats.TCPCurrent.Load()
	tcpPeak := s.Stats.TCPPeak.Load()
	udpCur := s.Stats.UDPCurrent.Load()
	udpPeak := s.Stats.UDPPeak.Load()
	normal := s.Stats.TCPCloseNormal.Load()
	tout := s.Stats.TCPCloseTimeout.Load()
	rst := s.Stats.TCPCloseReset.Load()
	hello := s.Stats.TCPCloseHello.Load()
	other := s.Stats.TCPCloseOther.Load()
	uptime := time.Since(s.StartTime).Round(time.Second)
	log.Printf("=== Final Stats (uptime: %s) ===", uptime)
	log.Printf("TCP: cur=%d peak=%d", tcpCur, tcpPeak)
	log.Printf("UDP: cur=%d peak=%d", udpCur, udpPeak)
	log.Printf("TCP close reasons: normal=%d timeout=%d reset=%d hello=%d other=%d",
		normal, tout, rst, hello, other)
}

func (s *Server) startHTTPServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"tcp_current":%d,"tcp_peak":%d,"udp_current":%d,"udp_peak":%d,"close_normal":%d,"close_timeout":%d,"close_reset":%d,"close_hello":%d,"close_other":%d,"uptime":"%s"}`,
			s.Stats.TCPCurrent.Load(),
			s.Stats.TCPPeak.Load(),
			s.Stats.UDPCurrent.Load(),
			s.Stats.UDPPeak.Load(),
			s.Stats.TCPCloseNormal.Load(),
			s.Stats.TCPCloseTimeout.Load(),
			s.Stats.TCPCloseReset.Load(),
			s.Stats.TCPCloseHello.Load(),
			s.Stats.TCPCloseOther.Load(),
			time.Since(s.StartTime).Round(time.Second).String(),
		)
	})
	addr := fmt.Sprintf(":%d", s.StatsPort)
	log.Printf("HTTP stats on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("HTTP server: %v", err)
	}
}

func (s *Server) initMultiIP() {
	ips, err := generateIPv6List(s.IPv6Prefix, s.MultiIPCount, s.reservedBindIP)
	if err != nil {
		log.Printf("multi-IP init failed: %v", err)
		return
	}
	s.multiIPList = ips
	log.Printf("multi-IP list generated: %d targets", len(ips))
	go s.prepareMultiIPListeners(ips)
}
