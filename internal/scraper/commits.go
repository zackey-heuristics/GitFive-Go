package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/errgroup"

	"github.com/zackey-heuristics/gitfive-go/internal/httpclient"
)

// CommitAccount holds the result of commit-based email discovery.
type CommitAccount struct {
	Avatar   string `json:"avatar"`
	FullName string `json:"full_name,omitempty"`
	Username string `json:"username"`
	IsTarget bool   `json:"is_target"`
}

var (
	embeddedDataRegex = regexp.MustCompile(`data-target="react-app\.embeddedData">(\{.*?\})</script>`)
	currentOidRegex   = regexp.MustCompile(`"currentOid":"(.*?)"`)
)

// commitPayload represents the embedded JSON structure on GitHub commits pages.
type commitPayload struct {
	Payload struct {
		CommitGroups []struct {
			Commits []commitEntry `json:"commits"`
		} `json:"commitGroups"`
	} `json:"payload"`
}

type commitEntry struct {
	OID     string         `json:"oid"`
	Authors []commitAuthor `json:"authors"`
}

type commitAuthor struct {
	DisplayName string `json:"displayName"`
	Login       string `json:"login"`
	AvatarURL   string `json:"avatarUrl"`
}

// ScrapeCommits analyzes commits in a repo to map emails to GitHub accounts.
// It fetches the mirage branch commits page(s) and parses the embedded React data.
func ScrapeCommits(ctx context.Context, client *httpclient.Client, owner, repoName string,
	emailsIndex map[string]string, targetUsername string, checkOnly bool, sem Semaphore) (map[string]*CommitAccount, error) {

	out := make(map[string]*CommitAccount)
	var mu sync.Mutex

	totalEmails := len(emailsIndex)
	if totalEmails == 0 {
		return out, nil
	}

	// Fetch the first commits page to get lastHash and the initial batch of commits.
	firstURL := fmt.Sprintf("https://github.com/%s/%s/commits/mirage", owner, repoName)
	resp, err := client.Get(ctx, firstURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	bodyHTML := ReadAll(resp)

	if strings.Contains(strings.ToLower(bodyHTML), "this repository is empty") {
		return nil, fmt.Errorf("empty repository")
	}

	// Get lastHash for pagination
	oidMatches := currentOidRegex.FindStringSubmatch(bodyHTML)
	if len(oidMatches) < 2 {
		return nil, fmt.Errorf("couldn't fetch last hash from commits page")
	}
	lastHash := oidMatches[1]

	// Parse the first page directly — this avoids needing commitCount.
	firstPageCommits := parseEmbeddedCommits(bodyHTML)

	// Process the first page inline
	processCommits(ctx, client, firstPageCommits, emailsIndex, targetUsername, checkOnly, &mu, out)

	// If there are more emails than commits on the first page, paginate.
	// GitHub shows ~35 commits per page.
	if totalEmails > len(firstPageCommits) && len(firstPageCommits) >= 35 {
		// Estimate how many more pages we need
		remaining := totalEmails - len(firstPageCommits)
		var pages []int
		for offset := 35; offset <= remaining+35; offset += 35 {
			pages = append(pages, offset)
		}

		bar := progressbar.NewOptions(len(pages),
			progressbar.OptionSetDescription("Fetching more commits..."),
			progressbar.OptionClearOnFinish(),
		)

		g, ctx := errgroup.WithContext(ctx)
		for _, page := range pages {
			page := page
			g.Go(func() error {
				if err := sem.Acquire(ctx, 1); err != nil {
					return err
				}
				defer sem.Release(1)

				url := fmt.Sprintf("https://github.com/%s/%s/commits/mirage?after=%s+%d&branch=mirage",
					owner, repoName, lastHash, page)

				resp, err := client.Get(ctx, url)
				if err != nil {
					return err
				}
				defer func() { _ = resp.Body.Close() }()

				if resp.StatusCode == 429 {
					return fmt.Errorf("rate-limit detected on commits scrape")
				}

				pageBody := ReadAll(resp)
				commits := parseEmbeddedCommits(pageBody)
				processCommits(ctx, client, commits, emailsIndex, targetUsername, checkOnly, &mu, out)

				_ = bar.Add(1)
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return nil, err
		}
		_ = bar.Finish()
	}

	return out, nil
}

// parseEmbeddedCommits extracts commit entries from the embedded React JSON.
func parseEmbeddedCommits(html string) []commitEntry {
	matches := embeddedDataRegex.FindStringSubmatch(html)
	if len(matches) < 2 {
		return nil
	}

	var payload commitPayload
	if err := json.Unmarshal([]byte(matches[1]), &payload); err != nil {
		return nil
	}

	if len(payload.Payload.CommitGroups) == 0 {
		return nil
	}

	// Collect commits from all groups (there may be multiple date groups)
	var all []commitEntry
	for _, group := range payload.Payload.CommitGroups {
		all = append(all, group.Commits...)
	}
	return all
}

// processCommits matches commits to emails and resolves GitHub accounts.
func processCommits(ctx context.Context, client *httpclient.Client,
	commits []commitEntry, emailsIndex map[string]string,
	targetUsername string, checkOnly bool,
	mu *sync.Mutex, out map[string]*CommitAccount) {

	for _, commit := range commits {
		if len(commit.Authors) == 0 {
			continue
		}

		// Find the author that is NOT gitfive_hunter and has a GitHub login.
		var target *commitAuthor
		for i := range commit.Authors {
			a := &commit.Authors[i]
			if a.DisplayName != "gitfive_hunter" && a.Login != "" {
				target = a
				break
			}
		}
		if target == nil {
			continue
		}

		email, ok := emailsIndex[commit.OID]
		if !ok {
			continue
		}

		account := &CommitAccount{
			Avatar:   target.AvatarURL,
			Username: target.Login,
			IsTarget: strings.EqualFold(target.Login, targetUsername),
		}

		if account.IsTarget {
			fmt.Printf("[+] [Target's email] %s -> @%s\n", email, target.Login)
		}

		if !checkOnly {
			name, _ := FetchProfileName(ctx, client, target.Login)
			account.FullName = name
		}

		mu.Lock()
		out[email] = account
		mu.Unlock()
	}
}
