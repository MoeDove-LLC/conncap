package server

import (
	"bufio"
	"io"
	"log"
	"net"
	"os"
	"syscall"
	"time"

	"conncap/internal/protocol"
)

func (s *Server) acceptTCP(ln *net.TCPListener) {
	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Printf("TCP accept error: %v", err)
			return
		}
		go s.handleTCP(conn)
	}
}

func (s *Server) handleTCP(conn *net.TCPConn) {
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(30 * time.Second)

	cur := s.Stats.TCPCurrent.Add(1)
	s.updateTCPPeak(cur)

	helloOK := false
	defer func() {
		s.Stats.TCPCurrent.Add(-1)
		conn.Close()
	}()

	conn.SetDeadline(time.Now().Add(protocol.HelloTimeout * time.Second))
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString(protocol.Delimiter)
	if err != nil {
		s.recordClose(err, helloOK)
		return
	}
	if line != protocol.MsgHello+string(protocol.Delimiter) {
		s.Stats.TCPCloseHello.Add(1)
		return
	}
	helloOK = true
	if _, err := conn.Write([]byte(protocol.MsgOK + string(protocol.Delimiter))); err != nil {
		s.recordClose(err, helloOK)
		return
	}

	for {
		conn.SetDeadline(time.Now().Add(60 * time.Second))
		line, err := reader.ReadString(protocol.Delimiter)
		if err != nil {
			s.recordClose(err, helloOK)
			return
		}
		if line == protocol.MsgPing+string(protocol.Delimiter) {
			if _, err := conn.Write([]byte(protocol.MsgPong + string(protocol.Delimiter))); err != nil {
				s.recordClose(err, helloOK)
				return
			}
		}
	}
}

func (s *Server) recordClose(err error, helloOK bool) {
	if !helloOK {
		s.Stats.TCPCloseHello.Add(1)
		return
	}
	if err == io.EOF {
		s.Stats.TCPCloseNormal.Add(1)
		return
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		s.Stats.TCPCloseTimeout.Add(1)
		return
	}
	if isConnReset(err) {
		s.Stats.TCPCloseReset.Add(1)
		return
	}
	s.Stats.TCPCloseOther.Add(1)
}

func isConnReset(err error) bool {
	if opErr, ok := err.(*net.OpError); ok {
		if sysErr, ok := opErr.Err.(*os.SyscallError); ok {
			return sysErr.Err == syscall.ECONNRESET
		}
		if opErr.Err == syscall.ECONNRESET {
			return true
		}
	}
	return false
}

func (s *Server) updateTCPPeak(cur int64) {
	for {
		peak := s.Stats.TCPPeak.Load()
		if cur <= peak {
			break
		}
		if s.Stats.TCPPeak.CompareAndSwap(peak, cur) {
			break
		}
	}
}
