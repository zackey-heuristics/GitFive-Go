package scraper

import (
	"testing"
)

func TestParseFollowCount(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"10 followers", 10},
		{" 1.2k following ", 1200},
		{"0", 0},
		{"", 0},
		{"\n 50 \n", 50},
	}
	for _, tt := range tests {
		got := parseFollowCount(tt.input)
		if got != tt.want {
			t.Errorf("parseFollowCount(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
