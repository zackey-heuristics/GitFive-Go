package util

import "testing"

func TestIsLocalDomain(t *testing.T) {
	tests := []struct {
		domain string
		want   bool
	}{
		{"localhost", true},
		{"myhost", true},
		{"machine.local", true},
		{"host.lan", true},
		{"example.com", false},
		{"github.com", false},
	}
	for _, tt := range tests {
		got := IsLocalDomain(tt.domain)
		if got != tt.want {
			t.Errorf("IsLocalDomain(%q) = %v, want %v", tt.domain, got, tt.want)
		}
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		url      string
		subLevel int
		want     string
	}{
		{"https://sub.example.com/path", 0, "example.com"},
		{"https://sub.example.com/path", 1, "sub.example.com"},
		{"example.com", 0, "example.com"},
		{"https://a.b.c.d.com/x", 0, "d.com"},
		{"https://a.b.c.d.com/x", 2, "b.c.d.com"},
	}
	for _, tt := range tests {
		got := ExtractDomain(tt.url, tt.subLevel)
		if got != tt.want {
			t.Errorf("ExtractDomain(%q, %d) = %q, want %q", tt.url, tt.subLevel, got, tt.want)
		}
	}
}

func TestDetectCustomDomain(t *testing.T) {
	tests := []struct {
		link string
		want int // expected number of domains
	}{
		{"https://example.com", 1},
		{"https://www.example.com", 1}, // www.example.com filtered, but example.com kept
		{"https://user.github.io", 0},  // github.io filtered
		{"nodot", 0},
	}
	for _, tt := range tests {
		got := DetectCustomDomain(tt.link)
		if len(got) != tt.want {
			t.Errorf("DetectCustomDomain(%q) returned %d domains %v, want %d", tt.link, len(got), got, tt.want)
		}
	}
}
