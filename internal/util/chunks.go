package util

// Chunks splits a slice into sub-slices of at most n elements.
func Chunks[T any](s []T, n int) [][]T {
	if n <= 0 {
		return nil
	}
	var result [][]T
	for i := 0; i < len(s); i += n {
		end := i + n
		if end > len(s) {
			end = len(s)
		}
		result = append(result, s[i:end])
	}
	return result
}
