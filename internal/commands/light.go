package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zackey-heuristics/gitfive-go/internal/runner"
)

// NewLightCmd creates the "light" subcommand for quick email discovery.
func NewLightCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "light <username>",
		Short: "Quickly find email addresses from a GitHub username",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			username := args[0]

			r, err := runner.New()
			if err != nil {
				return err
			}
			if err := r.Login(cmd.Context()); err != nil {
				return err
			}

			data, err := r.API.Query(cmd.Context(), fmt.Sprintf("/search/commits?q=author:%s&per_page=100&sort=author-date&order=asc", strings.ToLower(username)), "all")
			if err != nil {
				return err
			}

			var result struct {
				TotalCount int `json:"total_count"`
				Items      []struct {
					Commit struct {
						Author struct {
							Email string `json:"email"`
						} `json:"author"`
					} `json:"commit"`
				} `json:"items"`
			}
			_ = json.Unmarshal(data, &result)

			emails := make(map[string]bool)
			for _, item := range result.Items {
				email := item.Commit.Author.Email
				if email != "" && email != "noreply@github.com" && !strings.HasSuffix(email, "@users.noreply.github.com") {
					emails[email] = true
				}
			}

			// If >100 results, fetch from the other end
			if result.TotalCount > 100 {
				data2, err := r.API.Query(cmd.Context(), fmt.Sprintf("/search/commits?q=author:%s&per_page=100&sort=author-date&order=desc", strings.ToLower(username)), "all")
				if err == nil {
					var result2 struct {
						Items []struct {
							Commit struct {
								Author struct {
									Email string `json:"email"`
								} `json:"author"`
							} `json:"commit"`
						} `json:"items"`
					}
					_ = json.Unmarshal(data2, &result2)
					for _, item := range result2.Items {
						email := item.Commit.Author.Email
						if email != "" && email != "noreply@github.com" && !strings.HasSuffix(email, "@users.noreply.github.com") {
							emails[email] = true
						}
					}
				}
			}

			if len(emails) == 0 {
				fmt.Printf("\n[-] No email found for %s.\nYou should try full search!\n", username)
				return nil
			}

			fmt.Printf("\nEmail(s) found for user %q:\n", username)
			for email := range emails {
				fmt.Printf("- %s\n", email)
			}
			return nil
		},
	}
	return cmd
}
