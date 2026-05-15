package server

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

type Stats struct {
	TCPCurrent     atomic.Int64
	TCPPeak        atomic.Int64
	UDPCurrent     atomic.Int64
	UDPPeak        atomic.Int64
	TCPCloseNormal atomic.Int64
	TCPCloseTimeout atomic.Int64
	TCPCloseReset  atomic.Int64
	TCPCloseHello  atomic.Int64
	TCPCloseOther  atomic.Int64
}

type Server struct {
	Host      string
	TCPPort   int
	UDPPort   int
	StatsPort int
	Interval  time.Duration
	IPv6Only  bool

	Stats     Stats
	StartTime time.Time
}

func New(host string, tcpPort, udpPort, statsPort int, interval time.Duration, ipv6Only bool) *Server {
	return &Server{
		Host:      host,
		TCPPort:   tcpPort,
		UDPPort:   udpPort,
		StatsPort: statsPort,
		Interval:  interval,
		IPv6Only:  ipv6Only,
		StartTime: time.Now(),
	}
}

func (s *Server) Start() error {
	if s.StatsPort > 0 {
		go s.startHTTPServer()
	}

	go s.logStats()
	activeListeners := 0

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

func (s *Server) listenTCP(address, network string) error {
	tcpAddr, err := net.ResolveTCPAddr(network, address)
	if err != nil {
		return fmt.Errorf("resolve TCP %s: %w", network, err)
	}
	ln, err := net.ListenTCP(network, tcpAddr)
	if err != nil {
		return fmt.Errorf("listen TCP %s: %w", network, err)
	}
	log.Printf("TCP listening on %s (%s)", address, network)
	go s.acceptTCP(ln)
	return nil
}

func (s *Server) listenUDP(address, network string) error {
	udpAddr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return fmt.Errorf("resolve UDP %s: %w", network, err)
	}
	conn, err := net.ListenUDP(network, udpAddr)
	if err != nil {
		return fmt.Errorf("listen UDP %s: %w", network, err)
	}
	log.Printf("UDP listening on %s (%s)", address, network)
	go handleUDP(conn, &s.Stats)
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
