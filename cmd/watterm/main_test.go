package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/AamerAijaz/watterm/internal/queue"
)

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
}
