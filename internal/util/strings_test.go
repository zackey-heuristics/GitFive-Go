package util

import "testing"

func TestSanitize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "hello world"},
		{"café résumé", "cafe resume"},
		{"hello123", "hello"},
		{"john-doe", "johndoe"},
		{"Ünïcödé", "Unicode"},
		{"", ""},
	}
	for _, tt := range tests {
		got := Sanitize(tt.input)
		if got != tt.want {
			t.Errorf("Sanitize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestUnicodePatch(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"éèçà", "eeca"},
		{"hello", "hello"},
		{"café", "cafe"},
	}
	for _, tt := range tests {
		got := UnicodePatch(tt.input)
		if got != tt.want {
			t.Errorf("UnicodePatch(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSafePrint(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello", "hello"},
		{"[bold]text", "\\[bold]text"},
		{"normal\nline", "normal\nline"},
	}
	for _, tt := range tests {
		got := SafePrint(tt.input)
		if got != tt.want {
			t.Errorf("SafePrint(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHumanizeList(t *testing.T) {
	tests := []struct {
		input []string
		want  string
	}{
		{nil, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b"}, "a and b"},
		{[]string{"a", "b", "c"}, "a, b and c"},
		{[]string{"reader", "writer", "owner"}, "reader, writer and owner"},
	}
	for _, tt := range tests {
		got := HumanizeList(tt.input)
		if got != tt.want {
			t.Errorf("HumanizeList(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
