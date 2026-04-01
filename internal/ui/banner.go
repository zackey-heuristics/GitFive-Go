package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/zackey-heuristics/gitfive-go/internal/version"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	authorStyle = lipgloss.NewStyle().Bold(true)
	linkStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("38")) // deep sky blue
	heartStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("204")) // indian red
)

// ShowBanner prints the ASCII art banner and version info.
func ShowBanner() {
	banner := `
   ▄██████▄   ▄█      ███        ▄████████  ▄█   ▄█    █▄     ▄████████
  ███    ███ ███  ▀█████████▄   ███    ███ ███  ███    ███   ███    ███
  ███    █▀  ███▌    ▀███▀▀██   ███    █▀  ███▌ ███    ███   ███    █▀
 ▄███        ███▌     ███   ▀  ▄███▄▄▄     ███▌ ███    ███  ▄███▄▄▄
▀▀███ ████▄  ███▌     ███     ▀▀███▀▀▀     ███▌ ███    ███ ▀▀███▀▀▀
  ███    ███ ███      ███       ███         ███  ███    ███   ███    █▄
  ███    ███ ███      ███       ███         ███  ███    ███   ███    ███
  ████████▀  █▀      ▄████▀     ███         █▀    ▀██████▀    ██████████
`
	fmt.Println(banner)
	fmt.Println(titleStyle.Render(
		fmt.Sprintf("  GitFive-Go %s (%s)", version.Version, version.Name),
	))
	fmt.Println(authorStyle.Render("  By: mxrch (") +
		linkStyle.Render("@mxrchreborn") +
		authorStyle.Render(")"))
	fmt.Println(heartStyle.Render("  Support my work on GitHub Sponsors!"))
	fmt.Println()
}
