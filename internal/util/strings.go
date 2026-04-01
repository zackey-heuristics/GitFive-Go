package util

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/mozillazg/go-unidecode"
)

// Sanitize removes accents and non-alpha characters (except spaces) from text.
func Sanitize(text string) string {
	// First try direct unidecode
	deaccented := unidecode.Unidecode(text)

	// Filter: keep only ASCII letters and spaces
	var b strings.Builder
	for _, r := range deaccented {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == ' ' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// UnicodePatch replaces a small set of French accented characters.
func UnicodePatch(txt string) string {
	r := strings.NewReplacer("é", "e", "è", "e", "ç", "c", "à", "a")
	return r.Replace(txt)
}

// SafePrint escapes characters that could cause ANSI injection.
func SafePrint(txt string) string {
	var b strings.Builder
	for _, r := range txt {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			fmt.Fprintf(&b, "\\x%02x", r)
		} else if r == '[' {
			b.WriteString("\\[")
		} else if !unicode.IsPrint(r) && r != '\n' && r != '\r' && r != '\t' {
			fmt.Fprintf(&b, "\\u%04x", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// HumanizeList transforms a slice into a human-readable sentence.
// e.g. ["reader", "writer", "owner"] -> "reader, writer and owner"
func HumanizeList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
	}
}
