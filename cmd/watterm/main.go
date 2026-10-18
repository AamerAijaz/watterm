package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/mdp/qrterminal/v3"
	"golang.org/x/sys/windows"

	"github.com/AamerAijaz/watterm/internal/queue"
	"github.com/AamerAijaz/watterm/internal/server"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	dir, err := dataDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	enableVirtualTerminal()

	ui, srv := queue.New()
	link := server.NewWhatsApp(filepath.Join(dir, "session.db"))

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(runCtx, srv, link)
		cancel()
	}()

	for {
		state, err := ui.NextState(runCtx)
		if err != nil {
			break
		}
		printState(os.Stdout, state)
	}

	err = <-errCh
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func dataDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", errors.New("APPDATA is not set")
	}
	return filepath.Join(appData, "Watterm"), nil
}

func printState(w io.Writer, state queue.State) {
	// Clear the previous frame so a rotated code, and the code after linking, cannot be scanned.
	fmt.Fprint(w, "\033[H\033[2J\033[3J")
	switch state.Link {
	case queue.Linked:
		fmt.Fprintln(w, "linked")
	case queue.Pairing:
		fmt.Fprintln(w, "pairing")
		if state.QRCode != "" {
			qrterminal.GenerateHalfBlock(state.QRCode, qrterminal.L, w)
		}
	}
}

func enableVirtualTerminal() {
	handle := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return
	}
	_ = windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}
