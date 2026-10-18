package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/AamerAijaz/watterm/internal/link"
)

func TestFreshViewWaitsForQR(t *testing.T) {
	m := New(link.Initial(false))
	if !strings.Contains(m.View(), "Waiting for a QR code") {
		t.Fatalf("view = %q", m.View())
	}
}

func TestPairedRelaunchShowsLinked(t *testing.T) {
	m := New(link.Initial(true))
	view := m.View()
	if !strings.Contains(view, "Linked") {
		t.Fatalf("view = %q", view)
	}
	if strings.Contains(view, "Waiting for a QR code") {
		t.Fatalf("relaunch showed pairing: %q", view)
	}
}

func TestLinkedViewListsChatsInGivenOrder(t *testing.T) {
	m := apply(t, New(link.Initial(true)), ShowChats{
		{DisplayName: "Weekend"},
		{DisplayName: "Ada"},
	})
	view := m.View()
	weekend := strings.Index(view, "Weekend")
	ada := strings.Index(view, "Ada")
	if weekend < 0 || ada < 0 || weekend > ada {
		t.Fatalf("view = %q", view)
	}
}

func TestDisconnectReconnectsAndLogoutReturnsToQR(t *testing.T) {
	m := apply(t, New(link.Initial(false)), link.QR{Code: "pair-me"}, link.Connected{})
	if !strings.Contains(m.View(), "Linked") {
		t.Fatalf("view = %q", m.View())
	}

	m = apply(t, m, link.Disconnected{})
	view := m.View()
	if !strings.Contains(view, "Reconnecting") {
		t.Fatalf("view = %q", view)
	}
	if strings.Contains(view, "Waiting for a QR code") || strings.Contains(view, "Link this Companion") {
		t.Fatalf("reconnect looked like pairing: %q", view)
	}

	m = apply(t, m, link.LoggedOut{})
	if !strings.Contains(m.View(), "Waiting for a QR code") {
		t.Fatalf("view = %q", m.View())
	}
}

func apply(t *testing.T, m Model, msgs ...tea.Msg) Model {
	t.Helper()
	cur := m
	for _, msg := range msgs {
		next, _ := cur.Update(msg)
		cur = next.(Model)
	}
	return cur
}
