package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zackey-heuristics/gitfive-go/internal/analysis"
	"github.com/zackey-heuristics/gitfive-go/internal/runner"
	"github.com/zackey-heuristics/gitfive-go/internal/scraper"
)

// NewEmailCmd creates the "email" subcommand for reverse email lookup.
func NewEmailCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "email <email_address>",
		Short: "Track down a GitHub user by email address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			email := args[0]

			r, err := runner.New()
			if err != nil {
				return err
			}
			if err := r.Login(ctx); err != nil {
				return err
			}

			fmt.Printf("Looking up email: %s\n", email)

			tempRepoName, emailsIndex, err := analysis.StartMetamon(ctx, r.Client, r.Creds.Username, r.Creds.Token, []string{email})
			if err != nil {
				return fmt.Errorf("metamon failed: %w", err)
			}

			if len(emailsIndex) > 0 {
				accounts, err := scraper.ScrapeCommits(ctx, r.Client, r.Creds.Username, tempRepoName, emailsIndex, "", false, r.Limiters["commits_scrape"])
				if err != nil {
					fmt.Printf("[!] Commits scrape failed: %v\n", err)
				} else if acc, ok := accounts[email]; ok {
					fmt.Printf("[+] %s -> @%s", email, acc.Username)
					if acc.FullName != "" {
						fmt.Printf(" [%s]", acc.FullName)
					}
					fmt.Println()
				} else {
					fmt.Printf("[-] Email %s is not linked to any GitHub account.\n", email)
				}
			}

			// Cleanup
			if tempRepoName != "" {
				_ = scraper.DeleteRepo(ctx, r.Client, r.Creds.Username, tempRepoName, r.Creds.Password)
			}

			return nil
		},
	}

	return cmd
}
