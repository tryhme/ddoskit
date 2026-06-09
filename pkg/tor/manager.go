package tor

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Instance struct {
	ID          int
	SOCKSPort   int
	ControlPort int
	cmd         *exec.Cmd
	Ready       bool
}

type Manager struct {
	mu        sync.RWMutex
	instances []*Instance
	Count     int
	OnReady   func(ready, total int)
}

func NewManager(count int) *Manager {
	return &Manager{Count: count}
}

func (m *Manager) Start() {
	m.instances = make([]*Instance, m.Count)
	os.MkdirAll("/tmp/ddoskit", 0700)

	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ready := 0

	for i := 0; i < m.Count; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			inst := m.spawnInstance(idx)

			m.mu.Lock()
			m.instances[idx] = inst
			m.mu.Unlock()

			if inst.Ready {
				mu.Lock()
				ready++
				r := ready
				mu.Unlock()
				if m.OnReady != nil {
					m.OnReady(r, m.Count)
				}
			}
		}(i)
	}
	wg.Wait()
}

func (m *Manager) spawnInstance(idx int) *Instance {
	socksPort := 10000 + idx*2
	ctrlPort := 10001 + idx*2
	dataDir := filepath.Join("/tmp/ddoskit", fmt.Sprintf("t%d", idx))
	os.MkdirAll(dataDir, 0700)

	inst := &Instance{
		ID:          idx,
		SOCKSPort:   socksPort,
		ControlPort: ctrlPort,
	}

	cmd := exec.Command("tor",
		"--SocksPort", fmt.Sprintf("%d", socksPort),
		"--ControlPort", fmt.Sprintf("%d", ctrlPort),
		"--DataDirectory", dataDir,
		"--NewCircuitPeriod", "15",
		"--MaxCircuitDirtiness", "20",
		"--NumEntryGuards", "1",
		"--Log", "notice stdout",
	)

	pipe, err := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	if err != nil || cmd.Start() != nil {
		return inst
	}
	inst.cmd = cmd

	done := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "Bootstrapped 100%") {
				done <- true
				return
			}
		}
		done <- false
	}()

	select {
	case ok := <-done:
		inst.Ready = ok
	case <-time.After(45 * time.Second):
	}
	return inst
}

func (m *Manager) ActivePorts() []int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var ports []int
	for _, inst := range m.instances {
		if inst != nil && inst.Ready {
			ports = append(ports, inst.SOCKSPort)
		}
	}
	return ports
}

func (m *Manager) ActiveCount() int {
	return len(m.ActivePorts())
}

func (m *Manager) RotateAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, inst := range m.instances {
		if inst != nil && inst.Ready {
			go func(port int) {
				conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
				if err != nil {
					return
				}
				defer conn.Close()
				fmt.Fprintf(conn, "AUTHENTICATE \"\"\r\nSIGNAL NEWNYM\r\n")
			}(inst.ControlPort)
		}
	}
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, inst := range m.instances {
		if inst != nil && inst.cmd != nil && inst.cmd.Process != nil {
			inst.cmd.Process.Kill()
		}
	}
	os.RemoveAll("/tmp/ddoskit")
}
