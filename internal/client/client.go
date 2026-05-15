package client

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Stats struct {
	Attempt   atomic.Int64
	Success   atomic.Int64
	Failed    atomic.Int64
	Current   atomic.Int64
	PeakAlive atomic.Int64
}

type Config struct {
	ServerAddr string
	TCPPort    int
	UDPPort    int
	Protocol   string
	Max        int64
	Rate       int
	Timeout    time.Duration
	KeepAlive  time.Duration
	Duration   time.Duration
	IPVersion  string
	StopOnFail int
}

type Client struct {
	Config     Config
	Stats      Stats
	tcpNetwork string
	udpNetwork string
	startTime  time.Time
	done       chan struct{}
	doneOnce   sync.Once
}

func New(cfg Config) (*Client, error) {
	c := &Client{
		Config:    cfg,
		startTime: time.Now(),
		done:      make(chan struct{}),
	}

	tcpNet, udpNet, err := resolveNetworks(cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve network: %w", err)
	}
	c.tcpNetwork = tcpNet
	c.udpNetwork = udpNet

	return c, nil
}

func resolveNetworks(cfg Config) (string, string, error) {
	switch cfg.IPVersion {
	case "4":
		log.Printf("forced IPv4")
		return "tcp4", "udp4", nil
	case "6":
		log.Printf("forced IPv6")
		return "tcp6", "udp6", nil
	case "auto":
		return autoDetect(cfg)
	default:
		return "", "", fmt.Errorf("invalid IP version: %s (use auto, 4, or 6)", cfg.IPVersion)
	}
}

func autoDetect(cfg Config) (string, string, error) {
	addr := cfg.ServerAddr
	port := cfg.TCPPort
	if cfg.Protocol == "udp" {
		port = cfg.UDPPort
	}

	hasV6 := false
	hasV4 := false

	ips, err := net.DefaultResolver.LookupIPAddr(context.Background(), addr)
	if err != nil {
		if isLiteralIP(addr) {
			if isIPv6(addr) {
				hasV6 = true
			} else {
				hasV4 = true
			}
		} else {
			return "", "", fmt.Errorf("DNS resolve %s: %w", addr, err)
		}
	} else {
		for _, ip := range ips {
			if ip.IP.To4() == nil {
				hasV6 = true
			} else {
				hasV4 = true
			}
		}
	}

	if cfg.Protocol != "udp" && hasV6 {
		if probeConnect("tcp6", addr, port) {
			log.Printf("IPv6 probe OK, using IPv6")
			return "tcp6", "udp6", nil
		}
		log.Printf("IPv6 probe failed")
	}

	if cfg.Protocol != "udp" && hasV4 {
		if probeConnect("tcp4", addr, port) {
			log.Printf("IPv4 probe OK, using IPv4")
			return "tcp4", "udp4", nil
		}
		log.Printf("IPv4 probe failed")
	}

	if cfg.Protocol == "udp" {
		if hasV6 {
			log.Printf("UDP-only mode: using IPv6 from address resolution")
			return "tcp6", "udp6", nil
		}
		if hasV4 {
			log.Printf("UDP-only mode: using IPv4 from address resolution")
			return "tcp4", "udp4", nil
		}
	}

	if hasV6 {
		log.Printf("both probes failed, falling back to IPv6")
		return "tcp6", "udp6", nil
	}
	if hasV4 {
		log.Printf("falling back to IPv4")
		return "tcp4", "udp4", nil
	}
	return "", "", fmt.Errorf("no usable address for %s", addr)
}

func probeConnect(network, addr string, port int) bool {
	target := net.JoinHostPort(addr, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout(network, target, 3*time.Second)
	if err != nil {
		log.Printf("probe %s %s: %v", network, target, err)
		return false
	}
	conn.Close()
	return true
}

func isLiteralIP(s string) bool {
	return net.ParseIP(s) != nil
}

func isIPv6(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && ip.To4() == nil
}

func (c *Client) Start() error {
	if c.Config.Duration > 0 {
		go func() {
			timer := time.NewTimer(c.Config.Duration)
			defer timer.Stop()
			<-timer.C
			log.Printf("duration %v reached, stopping", c.Config.Duration)
			c.stop()
		}()
	}

	go c.logProgress()

	errCh := make(chan error, 2)
	var wg sync.WaitGroup

	switch c.Config.Protocol {
	case "tcp":
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.runTCP(); err != nil {
				errCh <- err
			}
		}()
	case "udp":
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.runUDP(); err != nil {
				errCh <- err
			}
		}()
	case "both":
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := c.runTCP(); err != nil {
				errCh <- err
			}
		}()
		go func() {
			defer wg.Done()
			if err := c.runUDP(); err != nil {
				errCh <- err
			}
		}()
	default:
		return fmt.Errorf("unknown protocol: %s (use tcp, udp, or both)", c.Config.Protocol)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		log.Printf("error: %v", err)
	}

	c.printFinalStats()
	return nil
}

func (c *Client) logProgress() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	var lastPeak int64
	var lastFailed int64
	var peakStallTicks int64
	var accumulatedFails int64
	for {
		select {
		case <-ticker.C:
			att := c.Stats.Attempt.Load()
			suc := c.Stats.Success.Load()
			fail := c.Stats.Failed.Load()
			cur := c.Stats.Current.Load()
			peak := c.Stats.PeakAlive.Load()
			log.Printf("attempt=%d success=%d failed=%d alive=%d peak=%d", att, suc, fail, cur, peak)

			if c.Config.StopOnFail > 0 {
				if peak == lastPeak {
					peakStallTicks++
					failDelta := fail - lastFailed
					accumulatedFails += failDelta
				} else {
					lastPeak = peak
					peakStallTicks = 0
					accumulatedFails = 0
				}
				lastFailed = fail

				if accumulatedFails >= int64(c.Config.StopOnFail) {
					log.Printf("failure wall: %d consecutive failures at peak=%d, stopping", accumulatedFails, peak)
					c.stop()
					return
				}
				if peakStallTicks >= int64(c.Config.StopOnFail) {
					log.Printf("peak plateau: peak=%d unchanged for %d seconds, stopping", peak, peakStallTicks)
					c.stop()
					return
				}
			}

			if c.Config.Max > 0 && suc >= c.Config.Max {
				c.stop()
				return
			}
		case <-c.done:
			return
		}
	}
}

func (c *Client) stop() {
	c.doneOnce.Do(func() {
		close(c.done)
	})
}

func (c *Client) printFinalStats() {
	att := c.Stats.Attempt.Load()
	suc := c.Stats.Success.Load()
	fail := c.Stats.Failed.Load()
	peak := c.Stats.PeakAlive.Load()
	uptime := time.Since(c.startTime).Round(time.Second)
	log.Printf("=== Final Stats (uptime: %s) ===", uptime)
	log.Printf("proto=%s attempts=%d success=%d failed=%d peak_alive=%d", c.Config.Protocol, att, suc, fail, peak)
}

func (c *Client) updatePeak(cur int64) {
	for {
		peak := c.Stats.PeakAlive.Load()
		if cur <= peak {
			break
		}
		if c.Stats.PeakAlive.CompareAndSwap(peak, cur) {
			break
		}
	}
}
