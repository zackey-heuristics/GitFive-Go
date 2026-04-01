package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/zackey-heuristics/gitfive-go/internal/commands"
	"github.com/zackey-heuristics/gitfive-go/internal/ui"
	"github.com/zackey-heuristics/gitfive-go/internal/version"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "gitfive",
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
