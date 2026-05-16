package server

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"conncap/internal/protocol"
)

const DefaultMultiIPCount = 32

func (s *Server) handleIPListRequest(conn *net.TCPConn, line string) {
	defer conn.Close()

	if s.IPv6Prefix == "" || len(s.multiIPList) == 0 {
		log.Printf("LIST denied: prefix=%q listLen=%d", s.IPv6Prefix, len(s.multiIPList))
		conn.Write([]byte(protocol.MsgNoIPList + string(protocol.Delimiter)))
		return
	}

	fields := strings.Fields(line)
	if len(fields) != 2 {
		log.Printf("LIST bad fields: %q", line)
		conn.Write([]byte(protocol.MsgNoIPList + string(protocol.Delimiter)))
		return
	}
	count, err := strconv.Atoi(fields[1])
	if err != nil || count <= 0 {
		log.Printf("LIST bad count: %s", fields[1])
		conn.Write([]byte(protocol.MsgNoIPList + string(protocol.Delimiter)))
		return
	}
	if count > len(s.multiIPList) {
		count = len(s.multiIPList)
	}

	firstIP := s.multiIPList[0]

	conn.SetDeadline(time.Time{})
	w := bufio.NewWriter(conn)
	fmt.Fprintf(w, "%s %s %d\n", protocol.MsgIPRange, firstIP, count)
	if err := w.Flush(); err != nil {
		log.Printf("multi-IP write response: %v", err)
		return
	}
	log.Printf("LIST range response sent: first=%s count=%d", firstIP, count)
	if err := conn.CloseWrite(); err != nil {
		log.Printf("multi-IP close write: %v", err)
		return
	}
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, _ = io.Copy(io.Discard, conn)
}

func generateIPv6List(prefix string, count int, reserved map[string]bool) ([]string, error) {
	baseIP, ipNet, err := net.ParseCIDR(prefix)
	if err != nil {
		return nil, err
	}
	base := ipNet.IP.To16()
	if base == nil || baseIP.To4() != nil {
		return nil, fmt.Errorf("prefix is not IPv6: %s", prefix)
	}
	ones, bits := ipNet.Mask.Size()
	if bits != 128 {
		return nil, fmt.Errorf("prefix is not IPv6: %s", prefix)
	}
	hostBits := 128 - ones
	capacity := new(big.Int).Lsh(big.NewInt(1), uint(hostBits))
	if big.NewInt(int64(count+len(reserved)+1)).Cmp(capacity) > 0 {
		return nil, fmt.Errorf("prefix %s does not contain %d usable addresses", prefix, count)
	}

	baseInt := new(big.Int).SetBytes(base)

	ips := make([]string, 0, count)
	for i := 1; len(ips) < count; i++ {
		v := new(big.Int).Add(baseInt, big.NewInt(int64(i)))
		b := v.Bytes()
		ip := make(net.IP, net.IPv6len)
		copy(ip[net.IPv6len-len(b):], b)
		ipText := ip.String()
		if reserved[ipText] {
			continue
		}
		ips = append(ips, ipText)
	}
	return ips, nil
}

func (s *Server) prepareMultiIPListeners(ips []string) {
	s.multiMu.Lock()
	defer s.multiMu.Unlock()

	sem := make(chan struct{}, 32)
	var wg sync.WaitGroup

	for _, ip := range ips {
		if s.multiIPs[ip] {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(addr string) {
			defer wg.Done()
			defer func() { <-sem }()
			var errs []string
			if s.TCPPort > 0 {
				if err := s.listenTCPWithLog(net.JoinHostPort(addr, strconv.Itoa(s.TCPPort)), "tcp6", false); err != nil {
					errs = append(errs, fmt.Sprintf("tcp: %v", err))
				}
			}
			if s.UDPPort > 0 {
				if err := s.listenUDPWithLog(net.JoinHostPort(addr, strconv.Itoa(s.UDPPort)), "udp6", false); err != nil {
					errs = append(errs, fmt.Sprintf("udp: %v", err))
				}
			}
			if len(errs) > 0 {
				log.Printf("multi-IP listener %s: %s", addr, strings.Join(errs, "; "))
			}
		}(ip)
	}

	wg.Wait()
	for _, ip := range ips {
		s.multiIPs[ip] = true
	}
	log.Printf("multi-IP listeners prepared: added=%d total=%d", len(ips), len(s.multiIPs))
}
