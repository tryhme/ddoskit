# ddoskit

Authorized stress testing framework for network resilience assessment.

## Features

- **HTTP/2 Rapid Reset** (CVE-2023-44487) — multiplexed request flooding
- **Slowloris** — connection exhaustion via partial HTTP headers
- **Cache Bust** — bypass CDN/reverse proxy caching with randomized parameters
- **TLS Flood** — repeated handshake initiation to exhaust server crypto resources
- **Tor integration** — automatic circuit rotation every 20s for source anonymization
- **Live TUI** — real-time dashboard with req/s, error rate, and target status

## Requirements

- Linux (WSL2 supported)
- Go 1.21+
- Tor

```bash
sudo apt install tor
```

## Usage

```bash
go build -o ddoskit .
./ddoskit
```

Enter the target URL when prompted. The tool verifies the target is reachable before starting and monitors availability throughout the test.

## Architecture

```
main.go
pkg/
  attacks/     rapidreset · slowloris · cachebust · tlsflood
  engine/      orchestrator — goroutine pool, rate limiting
  tor/         manager — instance lifecycle, circuit rotation
  ui/          tview-based terminal dashboard
  vpn/         pre-flight connectivity check
```

## Legal

For authorized penetration testing and network resilience assessment only. Do not use against systems you do not own or have explicit written permission to test.
