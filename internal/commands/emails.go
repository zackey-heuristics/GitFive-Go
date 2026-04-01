package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zackey-heuristics/gitfive-go/internal/analysis"
	"github.com/zackey-heuristics/gitfive-go/internal/runner"
	"github.com/zackey-heuristics/gitfive-go/internal/scraper"
)

// NewEmailsCmd creates the "emails" subcommand for batch email processing.
func NewEmailsCmd() *cobra.Command {
	var target string

	cmd := &cobra.Command{
		Use:   "emails <emails_file>",
		Short: "Find GitHub usernames for a list of email addresses",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			emailsFile := args[0]

			f, err := os.Open(emailsFile)
			if err != nil {
				return fmt.Errorf("couldn't open file: %w", err)
			}
			defer func() { _ = f.Close() }()

			var emails []string
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" && strings.Contains(line, "@") {
					emails = append(emails, line)
				}
			}

			if len(emails) == 0 {
				return fmt.Errorf("no valid emails found in %s", emailsFile)
			}

			r, err := runner.New()
			if err != nil {
				return err
			}
			if err := r.Login(ctx); err != nil {
				return err
			}

			fmt.Printf("Processing %d email(s)...\n", len(emails))

			tempRepoName, emailsIndex, err := analysis.StartMetamon(ctx, r.Client, r.Creds.Username, r.Creds.Token, emails)
			if err != nil {
				return fmt.Errorf("metamon failed: %w", err)
			}

			targetUsername := target
			if emailsIndex != nil {
				accounts, err := scraper.ScrapeCommits(ctx, r.Client, r.Creds.Username, tempRepoName, emailsIndex, targetUsername, false, r.Limiters["commits_scrape"])
				if err == nil {
					for email, acc := range accounts {
						marker := ""
						if acc.IsTarget {
							marker = " [TARGET]"
						}
						fmt.Printf("[+] %s -> @%s%s\n", email, acc.Username, marker)
					}
				}
			}

			// Cleanup
			if tempRepoName != "" {
				if err := scraper.DeleteRepo(ctx, r.Client, r.Creds.Username, tempRepoName, r.Creds.Password); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to delete temporary repository %s: %v\n", tempRepoName, err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "GitHub username of the target")
	return cmd
}
