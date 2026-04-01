package ui

import "testing"

func TestTMPrinter(t *testing.T) {
	p := NewTMPrinter()
	if p == nil {
		t.Fatal("NewTMPrinter returned nil")
	}
	// Smoke test — these write to stdout but shouldn't panic
	p.Out("hello")
	p.Out("hi") // shorter — should pad
	p.Clear()
}
