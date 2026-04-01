package util

import "testing"

func TestIsDiffLow(t *testing.T) {
	tests := []struct {
		s1, s2 string
		limit  int
		want   bool
	}{
		{"hello", "hello", 40, true},
		{"hello", "helo", 40, true},   // 1/5 = 20%
		{"hello", "world", 40, false}, // 4/5 = 80%
		{"abc", "abc", 0, true},
		{"", "", 40, true},
		{"a", "", 40, false}, // 1/1 = 100%
	}
	for _, tt := range tests {
		got := IsDiffLow(tt.s1, tt.s2, tt.limit)
		if got != tt.want {
			t.Errorf("IsDiffLow(%q, %q, %d) = %v, want %v", tt.s1, tt.s2, tt.limit, got, tt.want)
		}
	}
}
