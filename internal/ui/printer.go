package ui

import (
	"fmt"
	"strings"
)

// TMPrinter prints temporary text on the same line, overwriting previous output.
type TMPrinter struct {
	maxLen int
}

// NewTMPrinter creates a new TMPrinter.
func NewTMPrinter() *TMPrinter {
	return &TMPrinter{}
}

// Out prints text on the current line, padding to overwrite previous content.
func (p *TMPrinter) Out(text string) {
	if len(text) > p.maxLen {
		p.maxLen = len(text)
	} else {
		text += strings.Repeat(" ", p.maxLen-len(text))
	}
	fmt.Print("\r" + text)
}

// Clear clears the current line.
func (p *TMPrinter) Clear() {
	fmt.Print("\r" + strings.Repeat(" ", p.maxLen) + "\r")
	p.maxLen = 0
}
