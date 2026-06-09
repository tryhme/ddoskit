package main

import (
	"ddoskit/pkg/ui"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	app := ui.NewApp()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		app.Cleanup()
		os.Exit(0)
	}()

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	app.Cleanup()
}
