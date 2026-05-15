package client

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"sync/atomic"
	"time"

	"conncap/internal/protocol"
)

var keepaliveErrLogged atomic.Int64

func (c *Client) runTCP() error {
	var rateLimiter <-chan time.Time
	if c.Config.Rate > 0 {
		interval := time.Second / time.Duration(c.Config.Rate)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		rateLimiter = ticker.C
	}

	network := c.tcpNetwork
	addr := net.JoinHostPort(c.Config.ServerAddr, fmt.Sprintf("%d", c.Config.TCPPort))

	for {
		select {
		case <-c.done:
			for c.Stats.Current.Load() > 0 {
				time.Sleep(100 * time.Millisecond)
			}
			return nil
		default:
		}

		if c.Config.Max > 0 && c.Stats.Success.Load() >= c.Config.Max {
			c.stop()
			continue
		}

		if rateLimiter != nil {
			select {
			case <-c.done:
				continue
			case <-rateLimiter:
			}
		}
		c.Stats.Attempt.Add(1)

		go c.tcpConnect(network, addr)
	}
}

func (c *Client) tcpConnect(network, addr string) {
	conn, err := net.DialTimeout(network, addr, c.Config.Timeout)
	if err != nil {
		c.Stats.Failed.Add(1)
		return
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	conn.SetWriteDeadline(time.Now().Add(c.Config.Timeout))
	if _, err := conn.Write([]byte(protocol.MsgHello + string(protocol.Delimiter))); err != nil {
		conn.Close()
		c.Stats.Failed.Add(1)
		return
	}
	reader := bufio.NewReader(conn)
	conn.SetDeadline(time.Now().Add(protocol.HelloTimeout * time.Second))
	line, err := reader.ReadString(protocol.Delimiter)
	if err != nil || line != protocol.MsgOK+string(protocol.Delimiter) {
		conn.Close()
		c.Stats.Failed.Add(1)
		return
	}
	conn.SetDeadline(time.Time{})

	cur := c.Stats.Current.Add(1)
	c.updatePeak(cur)

	if c.Stats.Current.Load() > c.Config.Max && c.Config.Max > 0 {
		conn.Close()
		c.Stats.Current.Add(-1)
		return
	}

	c.Stats.Success.Add(1)

	c.tcpKeepAlive(conn, reader)
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetLinger(0)
	}
	conn.Close()
	c.Stats.Current.Add(-1)
}

func (c *Client) tcpKeepAlive(conn net.Conn, reader *bufio.Reader) {
	ticker := time.NewTicker(c.Config.KeepAlive)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
		}
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_, err := conn.Write([]byte(protocol.MsgPing + string(protocol.Delimiter)))
		if err != nil {
			if keepaliveErrLogged.Add(1) == 1 {
				log.Printf("keepalive write error (first, further suppressed): %v", err)
			}
			return
		}
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		line, err := reader.ReadString(protocol.Delimiter)
		if err != nil || line != protocol.MsgPong+string(protocol.Delimiter) {
			return
		}
		conn.SetDeadline(time.Time{})
	}
}
