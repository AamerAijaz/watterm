package ui

import (
	"bytes"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mdp/qrterminal/v3"

	"github.com/AamerAijaz/watterm/internal/link"
	"github.com/AamerAijaz/watterm/internal/transcript"
)

// ShowChats replaces the chat list.
type ShowChats []transcript.Chat

// Model is the Companion TUI.
type Model struct {
	snap  link.Snapshot
	chats []transcript.Chat
}

// New starts the TUI from a link snapshot.
func New(snap link.Snapshot) Model {
	return Model{snap: snap}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update folds link events into the screen.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case link.QR, link.Connected, link.Disconnected, link.LoggedOut:
		m.snap = m.snap.Apply(msg)
	case ShowChats:
		m.chats = []transcript.Chat(msg)
	}
	return m, nil
}

// View renders the current link phase.
func (m Model) View() string {
	switch m.snap.Phase {
	case link.PhaseLinked:
		var buf bytes.Buffer
		buf.WriteString("Linked\n")
		for _, chat := range m.chats {
			fmt.Fprintf(&buf, "%s\n", chat.DisplayName)
		}
		return buf.String()
	case link.PhaseReconnecting:
		return "Reconnecting\n"
	default:
		if m.snap.QR == "" {
			return "Link this Companion\nWaiting for a QR code\n"
		}
		var buf bytes.Buffer
		buf.WriteString("Link this Companion\n")
		qrterminal.GenerateHalfBlock(m.snap.QR, qrterminal.L, &buf)
		return buf.String()
	}
}
