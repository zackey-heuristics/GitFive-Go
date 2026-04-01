package ui

import (
	"fmt"

	"github.com/zackey-heuristics/gitfive-go/internal/version"
)

// ShowBanner prints the tool name and version.
func ShowBanner() {
	fmt.Printf("GitFive-Go %s\n\n", version.Version)
}
