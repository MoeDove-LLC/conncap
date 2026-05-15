package server

import (
	"log"
	"net"
	"sync"
	"time"

	"conncap/internal/protocol"
)

const udpSessionTimeout = 60 * time.Second

type udpSession struct {
	lastSeen time.Time
}

func handleUDP(conn *net.UDPConn, stats *Stats) {
	buf := make([]byte, 65535)
	sessions := make(map[string]*udpSession)
	var mu sync.Mutex

	go udpCleanupLoop(&mu, sessions, stats)

	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("UDP read error: %v", err)
			return
		}
		_ = n

		msg := string(buf[:n])
		if msg != protocol.MsgRegister+string(protocol.Delimiter) && msg != protocol.MsgPing+string(protocol.Delimiter) {
			continue
		}

		key := remoteAddr.String()

		mu.Lock()
		sess, exists := sessions[key]
		if !exists {
			sess = &udpSession{lastSeen: time.Now()}
			sessions[key] = sess
			cur := stats.UDPCurrent.Add(1)
			updateUDPPeak(stats, cur)
		}
		sess.lastSeen = time.Now()
		mu.Unlock()

		if msg == protocol.MsgRegister+string(protocol.Delimiter) {
			conn.WriteToUDP([]byte(protocol.MsgOK+string(protocol.Delimiter)), remoteAddr)
		} else if msg == protocol.MsgPing+string(protocol.Delimiter) {
			conn.WriteToUDP([]byte(protocol.MsgPong+string(protocol.Delimiter)), remoteAddr)
		}
	}
}

func udpCleanupLoop(mu *sync.Mutex, sessions map[string]*udpSession, stats *Stats) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		mu.Lock()
		now := time.Now()
		for key, sess := range sessions {
			if now.Sub(sess.lastSeen) > udpSessionTimeout {
				delete(sessions, key)
				stats.UDPCurrent.Add(-1)
			}
		}
		mu.Unlock()
	}
}

func updateUDPPeak(stats *Stats, cur int64) {
	for {
		peak := stats.UDPPeak.Load()
		if cur <= peak {
			break
		}
		if stats.UDPPeak.CompareAndSwap(peak, cur) {
			break
		}
	}
}
