package scraper

import (
	"context"
	"io"
	"net/http"

	"github.com/schollz/progressbar/v3"
)

// Semaphore abstracts a weighted semaphore for bounded concurrency.
type Semaphore interface {
	Acquire(ctx context.Context, n int64) error
	Release(n int64)
}

// ProgressBarWrapper wraps progressbar to allow safe concurrent Add calls.
type ProgressBarWrapper = progressbar.ProgressBar

// Response is an alias for http.Response used in scraper functions.
type Response = http.Response

// ReadAll reads the full body of a response and returns it as a string.
func ReadAll(resp *http.Response) string {
	data, _ := io.ReadAll(resp.Body)
	return string(data)
}
