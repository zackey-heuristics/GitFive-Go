package ui

import (
	"github.com/schollz/progressbar/v3"
)

// NewProgressBar creates a terminal progress bar with the given total and description.
func NewProgressBar(total int, description string) *progressbar.ProgressBar {
	return progressbar.NewOptions(total,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowCount(),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSetPredictTime(false),
	)
}
