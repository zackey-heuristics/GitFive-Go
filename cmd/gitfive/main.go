package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/zackey-heuristics/gitfive-go/internal/commands"
	"github.com/zackey-heuristics/gitfive-go/internal/gitcred"
	"github.com/zackey-heuristics/gitfive-go/internal/ui"
	"github.com/zackey-heuristics/gitfive-go/internal/version"
)

func main() {
	// Short-circuit when this binary is invoked as a GIT_ASKPASS helper by
	// a child git process we spawned. Must run before cobra parses argv,
	// because git passes a prompt string (not a known subcommand) as
	// os.Args[1], which cobra would otherwise reject as unknown.
	if handled, err := gitcred.HandleAskPassMode(); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	rootCmd := &cobra.Command{
		Use:   "gitfive-go",
		Short: "Track down GitHub users",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			ui.ShowBanner()
		},
		Version: fmt.Sprintf("%s (%s)", version.Version, version.Name),
	}

	rootCmd.AddCommand(
		commands.NewLoginCmd(),
		commands.NewUserCmd(),
		commands.NewEmailCmd(),
		commands.NewEmailsCmd(),
		commands.NewLightCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
