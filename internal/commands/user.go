package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zackey-heuristics/gitfive-go/internal/analysis"
	"github.com/zackey-heuristics/gitfive-go/internal/config"
	"github.com/zackey-heuristics/gitfive-go/internal/models"
	"github.com/zackey-heuristics/gitfive-go/internal/runner"
	"github.com/zackey-heuristics/gitfive-go/internal/scraper"
	"github.com/zackey-heuristics/gitfive-go/internal/util"
)

// NewUserCmd creates the "user" subcommand for full reconnaissance.
func NewUserCmd() *cobra.Command {
	var jsonFile string

	cmd := &cobra.Command{
		Use:   "user <username>",
		Short: "Track down a GitHub user by username",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			username := args[0]

			r, err := runner.New()
			if err != nil {
				return err
			}
			if err := r.Login(ctx); err != nil {
				return err
			}

			// Fetch user profile
			data, err := r.API.Query(ctx, fmt.Sprintf("/users/%s", username), "all")
			if err != nil {
				return err
			}

			var profile map[string]interface{}
			if err := json.Unmarshal(data, &profile); err != nil {
				return fmt.Errorf("failed to parse user profile: %w", err)
			}
			if msg, _ := profile["message"].(string); msg == "Not Found" {
				return fmt.Errorf("user %q not found", username)
			}

			scrapeProfile(r.Target, profile)

			if r.Target.Type == "Organization" {
				return fmt.Errorf("GitFive doesn't support organizations yet")
			}

			// Profile display
			fmt.Println("\nPROFILE")
			fmt.Println("\n[Identifiers]")
			fmt.Printf("Username: %s\n", r.Target.Username)
			if r.Target.Name != "" {
				fmt.Printf("Name: %s\n", r.Target.Name)
			}
			fmt.Printf("ID: %d\n", r.Target.ID)

			fmt.Println("\n[Stats]")
			fmt.Printf("Public repos: %d\n", r.Target.NbPublicRepos)
			fmt.Printf("Followers: %d\n", r.Target.NbFollowers)
			fmt.Printf("Following: %d\n", r.Target.NbFollowing)

			if r.Target.Company != "" {
				fmt.Printf("Company: %s\n", r.Target.Company)
			}
			if r.Target.Location != "" {
				fmt.Printf("Location: %s\n", r.Target.Location)
			}
			if r.Target.Blog != "" {
				fmt.Printf("Blog: %s\n", r.Target.Blog)
			}
			if r.Target.Twitter != "" {
				fmt.Printf("Twitter: @%s\n", r.Target.Twitter)
			}

			// Add names for generation
			r.Target.AddName(r.Target.Username)
			r.Target.AddName(r.Target.Twitter)
			r.Target.AddName(r.Target.Name)

			// Repos
			fmt.Println("\nREPOSITORIES")
			repos, err := scraper.FetchReposList(ctx, r.Client, r.Target, r.Limiters["repos_list"])
			if err != nil {
				fmt.Printf("[!] Failed to fetch repos: %v\n", err)
			} else {
				r.Target.Repos = repos
				r.Target.LanguagesStats = scraper.ComputeLanguageStats(repos)
				scraper.ShowRepos(r.Target)
			}

			// Close friends
			fmt.Println("\nCLOSE FRIENDS")
			friends, err := analysis.GuessCloseFriends(ctx, r.Client, r.Target, r.Limiters["social_follows"])
			if err != nil {
				fmt.Printf("[!] Failed to analyze close friends: %v\n", err)
			} else {
				r.Target.PotentialFriends = friends
				analysis.ShowCloseFriends(friends)
			}

			// Organizations
			fmt.Println("\nORGANIZATIONS")
			orgs, err := scraper.ScrapeOrgs(ctx, r.Client, r.Target.Username, r.Limiters["orgs_list"])
			if err != nil {
				fmt.Printf("[!] Failed to fetch orgs: %v\n", err)
			} else {
				scraper.ShowOrgs(orgs)
				// Add org domains
				for _, org := range orgs {
					for _, dom := range org.WebsiteDomains {
						r.Target.Domains.Add(dom)
					}
				}
			}

			// Company domain
			if r.Target.Company != "" {
				companyDomains := analysis.GuessCustomDomain(ctx, r.Client, r.Target)
				for d := range companyDomains {
					for _, cd := range util.DetectCustomDomain(d) {
						r.Target.Domains.Add(cd)
					}
				}
			}

			// Blog domain
			if r.Target.Blog != "" {
				for _, d := range util.DetectCustomDomain(r.Target.Blog) {
					if !r.Target.Domains.Contains(d) {
						fmt.Printf("[+] Found possible personal domain: %s\n", d)
						r.Target.Domains.Add(d)
					}
				}
			}

			// XRAY analysis
			fmt.Println("\nIDENTITIES UNMASKING")
			results, err := analysis.XrayAnalyze(ctx, r.Creds.Token, r.Target.Username, r.Target.ID, r.Target.Repos)
			if err != nil {
				fmt.Printf("[!] XRAY failed: %v\n", err)
			} else {
				mergeXrayResults(r.Target, results)
			}

			// Metamon + commits for discovered emails
			emailCandidates := make([]string, 0)
			for email := range r.Target.AllContribs {
				if email != "noreply@github.com" && !strings.HasSuffix(email, "@users.noreply.github.com") {
					emailCandidates = append(emailCandidates, email)
				}
			}

			if len(emailCandidates) > 0 {
				fmt.Println("[XRAY] Impersonating users from dumped commits...")
				tempRepoName, emailsIndex, err := analysis.StartMetamon(ctx, r.Client, r.Creds.Username, r.Creds.Token, emailCandidates)
				if err != nil {
					fmt.Printf("[!] Metamon failed: %v\n", err)
				} else if emailsIndex != nil {
					accounts, err := scraper.ScrapeCommits(ctx, r.Client, r.Creds.Username, tempRepoName, emailsIndex, r.Target.Username, true, r.Limiters["commits_scrape"])
					if err == nil {
						for email, acc := range accounts {
							r.EmailsAccounts[email] = map[string]interface{}{
								"username":  acc.Username,
								"is_target": acc.IsTarget,
							}
						}
					}
					_ = scraper.DeleteRepo(ctx, r.Client, r.Creds.Username, tempRepoName, r.Creds.Password)
				}
			}

			// Near names
			iteration := 0
			analysis.NearLookup(r.Target, &iteration)

			// Iterative email generation
			fmt.Println("\nEMAIL GENERATION")
			var lastTempRepo string
			for {
				emails := analysis.GenerateEmails(r.Target, r.SpoofedEmails,
					nil, config.EmailsDefaultDomains, config.EmailCommonDomainsPrefixes)

				for _, e := range emails {
					r.Target.GeneratedEmails.Add(e)
				}

				if len(emails) == 0 {
					fmt.Println("[-] No more emails have been generated.")
					break
				}
				fmt.Printf("[+] %d potential email(s) generated!\n", len(emails))

				tempRepoName, emailsIndex, err := analysis.StartMetamon(ctx, r.Client, r.Creds.Username, r.Creds.Token, emails)
				lastTempRepo = tempRepoName
				if err != nil {
					fmt.Printf("[!] Metamon failed: %v\n", err)
					break
				}
				if emailsIndex == nil {
					break
				}

				accounts, err := scraper.ScrapeCommits(ctx, r.Client, r.Creds.Username, tempRepoName, emailsIndex, r.Target.Username, false, r.Limiters["commits_scrape"])
				if err != nil {
					fmt.Printf("[!] Commits scrape failed: %v\n", err)
					break
				}

				if len(accounts) == 0 {
					fmt.Println("[-] No email matched a GitHub account.")
					break
				}

				// Display found accounts
				for email, acc := range accounts {
					marker := ""
					if acc.IsTarget {
						marker = " [TARGET]"
					}
					nameStr := ""
					if acc.FullName != "" {
						nameStr = fmt.Sprintf(" [%s]", acc.FullName)
					}
					fmt.Printf("[+] %s -> @%s%s%s\n", email, acc.Username, nameStr, marker)
				}

				newUsernames := false
				for email, acc := range accounts {
					if acc.IsTarget {
						r.Target.Emails.Add(email)
						handle := strings.SplitN(email, "@", 2)[0]
						handle = strings.SplitN(handle, "+", 2)[0]
						if !r.Target.Usernames.Contains(handle) {
							newUsernames = true
							r.Target.AddName(handle)
							fmt.Printf("[+] New valid username: %s\n", handle)
						}
					}
				}

				if !newUsernames {
					break
				}

				newVariations := analysis.NearLookup(r.Target, &iteration)
				if !newVariations {
					fmt.Println("[-] No more name variations found.")
					break
				}
			}

			// Cleanup
			if lastTempRepo != "" {
				_ = scraper.DeleteRepo(ctx, r.Client, r.Creds.Username, lastTempRepo, r.Creds.Password)
				fmt.Println("[+] Deleted the remote repo")
			}

			// Summary
			fmt.Println("\nRESULTS SUMMARY")
			if len(r.Target.Emails) > 0 {
				fmt.Printf("[+] %d confirmed email(s) for %s:\n", len(r.Target.Emails), r.Target.Username)
				for email := range r.Target.Emails {
					fmt.Printf("  - %s\n", email)
				}
			} else {
				fmt.Printf("[-] No confirmed emails found for %s.\n", r.Target.Username)
			}
			if len(r.Target.Usernames) > 1 {
				fmt.Printf("[+] Known usernames: %s\n", strings.Join(r.Target.Usernames.Slice(), ", "))
			}
			if len(r.Target.Fullnames) > 0 {
				fmt.Printf("[+] Known names: %s\n", strings.Join(r.Target.Fullnames.Slice(), ", "))
			}

			// JSON export
			if jsonFile != "" {
				jsonOutput, err := r.Target.ExportJSON()
				if err != nil {
					return fmt.Errorf("JSON export failed: %w", err)
				}
				if err := os.WriteFile(jsonFile, []byte(jsonOutput), 0o644); err != nil {
					return err
				}
				fmt.Printf("[+] JSON output wrote to %s!\n", jsonFile)
			}

			r.TMPrinter.Out("Deleting temp folder...")
			_ = util.DeleteTmpDir()
			r.TMPrinter.Clear()

			return nil
		},
	}

	cmd.Flags().StringVar(&jsonFile, "json", "", "File to write the JSON output to")
	return cmd
}

func scrapeProfile(target *models.Target, data map[string]interface{}) {
	if v, ok := data["login"].(string); ok {
		target.Username = v
	}
	if v, ok := data["name"].(string); ok {
		target.Name = util.UnicodePatch(v)
	}
	if v, ok := data["id"].(float64); ok {
		target.ID = int(v)
	}
	if v, ok := data["type"].(string); ok {
		target.Type = v
	}
	if v, ok := data["site_admin"].(bool); ok {
		target.IsSiteAdmin = v
	}
	if v, ok := data["hireable"].(bool); ok {
		target.Hireable = v
	}
	if v, ok := data["company"].(string); ok {
		target.Company = v
	}
	if v, ok := data["blog"].(string); ok {
		target.Blog = v
	}
	if v, ok := data["location"].(string); ok {
		target.Location = v
	}
	if v, ok := data["bio"].(string); ok {
		target.Bio = v
	}
	if v, ok := data["twitter_username"].(string); ok {
		target.Twitter = v
	}
	if v, ok := data["public_repos"].(float64); ok {
		target.NbPublicRepos = int(v)
	}
	if v, ok := data["followers"].(float64); ok {
		target.NbFollowers = int(v)
	}
	if v, ok := data["following"].(float64); ok {
		target.NbFollowing = int(v)
	}
	if v, ok := data["avatar_url"].(string); ok {
		target.AvatarURL = strings.SplitN(v, "?", 2)[0]
	}
}

func mergeXrayResults(target *models.Target, results []*analysis.RepoAnalysisResult) {
	if results == nil {
		return
	}
	for _, r := range results {
		for username, entry := range r.UsernamesHistory {
			target.AddName(username)
			if _, ok := target.UsernamesHistory[username]; !ok {
				target.UsernamesHistory[username] = entry
			} else {
				for name := range entry.Names {
					target.UsernamesHistory[username].Names.Add(name)
				}
			}
		}
		for email, entry := range r.AllContribs {
			if _, ok := target.AllContribs[email]; !ok {
				target.AllContribs[email] = entry
			} else {
				for name := range entry.Names {
					target.AllContribs[email].Names.Add(name)
				}
			}
		}
		for email, entry := range r.InternalContribs {
			if _, ok := target.InternalContribs.All[email]; !ok {
				target.InternalContribs.All[email] = entry
			} else {
				for name := range entry.Names {
					target.InternalContribs.All[email].Names.Add(name)
				}
			}
		}
	}
}
