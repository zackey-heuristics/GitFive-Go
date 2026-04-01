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
	"github.com/zackey-heuristics/gitfive-go/internal/ui"
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
	commitCountRegex  = regexp.MustCompile(`"commitCount":"(.*?)"`)
)

// ScrapeCommits analyzes commits in a repo to map emails to GitHub accounts.
func ScrapeCommits(ctx context.Context, client *httpclient.Client, owner, repoName string,
	emailsIndex map[string]string, targetUsername string, checkOnly bool, sem Semaphore) (map[string]*CommitAccount, error) {

	out := make(map[string]*CommitAccount)
	var mu sync.Mutex

	// Get the mirage branch commits page to find last hash and commit count.
	// We use the commits page directly (not the repo root) because metamon
	// pushes to the "mirage" branch which may not be the default branch.
	resp, err := client.Get(ctx, fmt.Sprintf("https://github.com/%s/%s/commits/mirage", owner, repoName))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyHTML := ReadAll(resp)

	// Check if empty
	if strings.Contains(strings.ToLower(bodyHTML), "this repository is empty") {
		return nil, fmt.Errorf("empty repository")
	}

	// Get last hash from the commits page
	oidMatches := currentOidRegex.FindStringSubmatch(bodyHTML)
	if len(oidMatches) < 2 {
		// Fallback: try the repo root page
		resp2, err := client.Get(ctx, fmt.Sprintf("https://github.com/%s/%s", owner, repoName))
		if err != nil {
			return nil, fmt.Errorf("couldn't fetch last hash")
		}
		defer resp2.Body.Close()
		bodyHTML2 := ReadAll(resp2)
		oidMatches = currentOidRegex.FindStringSubmatch(bodyHTML2)
		if len(oidMatches) < 2 {
			return nil, fmt.Errorf("couldn't fetch last hash")
		}
		bodyHTML = bodyHTML2
	}
	lastHash := oidMatches[1]

	// Get total commits
	total := 0
	countMatches := commitCountRegex.FindStringSubmatch(bodyHTML)
	if len(countMatches) >= 2 {
		countStr := strings.ReplaceAll(countMatches[1], ",", "")
		if countStr == "\u221e" { // infinity symbol
			total = 50000
		} else {
			total = parseCount(countStr)
		}
	}

	if total == 0 {
		return out, nil
	}

	// Build page offsets
	pages := []int{0}
	for i := 34; i < total; i += 35 {
		pages = append(pages, i)
	}

	bar := ui.NewProgressBar(total, "Fetching committers...")

	g, ctx := errgroup.WithContext(ctx)
	for _, page := range pages {
		page := page
		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			return fetchCommitsPage(ctx, client, owner, repoName, emailsIndex, lastHash, page, targetUsername, checkOnly, &mu, out, bar)
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	bar.Finish()
	return out, nil
}

func fetchCommitsPage(ctx context.Context, client *httpclient.Client, owner, repoName string,
	emailsIndex map[string]string, lastHash string, page int, targetUsername string,
	checkOnly bool, mu *sync.Mutex, out map[string]*CommitAccount, bar *progressbar.ProgressBar) error {

	var url string
	if page == 0 {
		url = fmt.Sprintf("https://github.com/%s/%s/commits/mirage", owner, repoName)
	} else {
		url = fmt.Sprintf("https://github.com/%s/%s/commits/mirage?after=%s+%d&branch=mirage", owner, repoName, lastHash, page)
	}

	resp, err := client.Get(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return fmt.Errorf("rate-limit detected on commits scrape")
	}

	bodyHTML := ReadAll(resp)
	matches := embeddedDataRegex.FindStringSubmatch(bodyHTML)
	if len(matches) < 2 {
		return nil
	}

	var payload struct {
		Payload struct {
			CommitGroups []struct {
				Commits []struct {
					OID     string `json:"oid"`
					Authors []struct {
						DisplayName string `json:"displayName"`
						Login       string `json:"login"`
						AvatarURL   string `json:"avatarUrl"`
					} `json:"authors"`
				} `json:"commits"`
			} `json:"commitGroups"`
		} `json:"payload"`
	}

	if err := json.Unmarshal([]byte(matches[1]), &payload); err != nil {
		return nil
	}

	if len(payload.Payload.CommitGroups) == 0 {
		return nil
	}

	for _, commit := range payload.Payload.CommitGroups[0].Commits {
		if len(commit.Authors) < 2 {
			continue
		}

		var targetAuthor *struct {
			DisplayName string
			Login       string
			AvatarURL   string
		}
		for i, a := range commit.Authors {
			if a.DisplayName != "gitfive_hunter" && a.Login != "" {
				targetAuthor = &struct {
					DisplayName string
					Login       string
					AvatarURL   string
				}{a.DisplayName, a.Login, a.AvatarURL}
				_ = i
				break
			}
		}
		if targetAuthor == nil {
			continue
		}

		email, ok := emailsIndex[commit.OID]
		if !ok {
			continue
		}

		account := &CommitAccount{
			Avatar:   targetAuthor.AvatarURL,
			Username: targetAuthor.Login,
			IsTarget: strings.EqualFold(targetAuthor.Login, targetUsername),
		}

		if account.IsTarget {
			fmt.Printf("[+] [Target's email] %s -> @%s\n", email, targetAuthor.Login)
		}

		if !checkOnly {
			name, _ := FetchProfileName(ctx, client, targetAuthor.Login)
			account.FullName = name
		}

		mu.Lock()
		out[email] = account
		mu.Unlock()
	}

	bar.Add(35)
	return nil
}

