package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/AamerAijaz/watterm/internal/companion"
	"github.com/AamerAijaz/watterm/internal/instance"
	"github.com/AamerAijaz/watterm/internal/link"
	"github.com/AamerAijaz/watterm/internal/paths"
	"github.com/AamerAijaz/watterm/internal/transcript"
	"github.com/AamerAijaz/watterm/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run() error {
	lay, err := paths.Open(paths.DefaultRoot())
	if err != nil {
		return err
	}
	lock, err := instance.Acquire(lay.Lock)
	if err != nil {
		return err
	}
	defer lock.Release()

	ctx := context.Background()
	running, err := companion.Open(ctx, lay)
	if err != nil {
		return err
	}
	defer running.Close()

	record, err := transcript.Open(lay.Transcript)
	if err != nil {
		return err
	}
	defer record.Close()

	events, err := running.Start(ctx, record)
	if err != nil {
		return err
	}
	chats, err := record.List()
	if err != nil {
		return err
	}
	model, _ := ui.New(running.Snapshot()).Update(ui.ShowChats(chats))

	program := tea.NewProgram(model, tea.WithAltScreen())
	go forward(program, record, events, func() {
		running.Close()
		_ = paths.RemoveSession(lay)
	})
	_, err = program.Run()
	return err
}

func forward(program *tea.Program, record *transcript.Store, events <-chan any, onLogout func()) {
	for ev := range events {
		if _, ok := ev.(companion.TranscriptChanged); ok {
			chats, err := record.List()
			if err != nil {
				continue
			}
			program.Send(ui.ShowChats(chats))
			continue
		}
		program.Send(ev)
		if _, ok := ev.(link.LoggedOut); ok {
			onLogout()
		}
	}
}
