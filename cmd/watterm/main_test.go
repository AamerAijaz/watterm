package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/AamerAijaz/watterm/internal/queue"
)

func TestSecondProcessAgainstTheSameAppDataIsRefused(t *testing.T) {
	dir := t.TempDir()
	release, err := acquire(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = acquire(dir)
	if !errors.Is(err, errAlreadyRunning) {
		t.Fatalf("second acquire = %v, want one Companion is already running", err)
	}

	other, err := acquire(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	other()

	release()
	release, err = acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestPrintState(t *testing.T) {
	var buf bytes.Buffer

	printState(&buf, queue.State{Link: queue.Pairing})
	if buf.String() != "\033[H\033[2J\033[3Jpairing\n" {
		t.Fatalf("pairing = %q", buf.String())
	}

	buf.Reset()
	printState(&buf, queue.State{Link: queue.Pairing, QRCode: "111"})
	const frame = "\033[H\033[2J\033[3Jpairing\n"
	if !strings.HasPrefix(buf.String(), frame) || len(buf.String()) <= len(frame) {
		t.Fatalf("pairing with a code did not print a QR, got %q", buf.String())
	}

	buf.Reset()
	printState(&buf, queue.State{Link: queue.Pairing, QRCode: "stale"})
	printState(&buf, queue.State{Link: queue.Linked, QRCode: "stale"})
	out := buf.String()
	if strings.Count(out, "\033[H\033[2J\033[3J") != 2 || !strings.HasSuffix(out, "linked\n") {
		t.Fatalf("latest frame did not replace the previous one: %q", out)
	}

	buf.Reset()
	printState(&buf, queue.State{Link: queue.Reconnecting, QRCode: "111"})
	if buf.String() != "\033[H\033[2J\033[3Jreconnecting\n" {
		t.Fatalf("reconnecting = %q, want reconnecting and no QR", buf.String())
	}
}
