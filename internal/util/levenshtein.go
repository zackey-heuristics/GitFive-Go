package util

import "github.com/agnivade/levenshtein"

// IsDiffLow returns true if the Levenshtein distance between two strings,
// expressed as a percentage of the first string's length, is at or below limit%.
func IsDiffLow(s1, s2 string, limit int) bool {
	if len(s1) == 0 {
		return len(s2) == 0
	}
	dist := levenshtein.ComputeDistance(s1, s2)
	pct := dist * 100 / len(s1)
	return pct <= limit
}
