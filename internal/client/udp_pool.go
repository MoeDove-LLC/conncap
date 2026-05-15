package client

import (
	"fmt"
	"net"
	"time"

	"conncap/internal/protocol"
)

func (c *Client) runUDP() error {
	network := c.udpNetwork

	var rateLimiter <-chan time.Time
	if c.Config.Rate > 0 {
		interval := time.Second / time.Duration(c.Config.Rate)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		rateLimiter = ticker.C
	}

	serverAddr := net.JoinHostPort(c.Config.ServerAddr, fmt.Sprintf("%d", c.Config.UDPPort))

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

		go c.udpSession(network, serverAddr)
	}
}

func (c *Client) udpSession(network, addr string) {
	resolved, err := net.ResolveUDPAddr(network, addr)
	if err != nil {
		c.Stats.Failed.Add(1)
		return
	}

	conn, err := net.DialUDP(network, nil, resolved)
	if err != nil {
		c.Stats.Failed.Add(1)
		return
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(protocol.MsgRegister + string(protocol.Delimiter))); err != nil {
		c.Stats.Failed.Add(1)
		return
	}
	conn.SetReadDeadline(time.Now().Add(c.Config.Timeout))
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil || string(buf[:n]) != protocol.MsgOK+string(protocol.Delimiter) {
		c.Stats.Failed.Add(1)
		return
	}
	conn.SetReadDeadline(time.Time{})

	cur := c.Stats.Current.Add(1)
	c.updatePeak(cur)
	c.Stats.Success.Add(1)

	ticker := time.NewTicker(c.Config.KeepAlive)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			c.Stats.Current.Add(-1)
			return
		case <-ticker.C:
		}
		_, err := conn.Write([]byte(protocol.MsgPing + string(protocol.Delimiter)))
		if err != nil {
			c.Stats.Current.Add(-1)
			return
		}
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := conn.Read(buf)
		if err != nil || string(buf[:n]) != protocol.MsgPong+string(protocol.Delimiter) {
			c.Stats.Current.Add(-1)
			return
		}
		conn.SetReadDeadline(time.Time{})
	}
}
