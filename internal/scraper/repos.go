package scraper

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/sync/errgroup"

	"github.com/zackey-heuristics/gitfive-go/internal/httpclient"
	"github.com/zackey-heuristics/gitfive-go/internal/models"
	"github.com/zackey-heuristics/gitfive-go/internal/ui"
)

// FetchReposList fetches all repositories for the given username.
func FetchReposList(ctx context.Context, client *httpclient.Client, target *models.Target, sem Semaphore) ([]models.RepoDetails, error) {
	totalRepos := target.NbPublicRepos
	totalPages := int(math.Ceil(float64(totalRepos) / 30.0))

	var mu sync.Mutex
	var repos []models.RepoDetails

	bar := ui.NewProgressBar(totalRepos, "Fetching repos...")

	g, ctx := errgroup.WithContext(ctx)
	for page := 1; page <= totalPages; page++ {
		page := page
		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			url := fmt.Sprintf("https://github.com/%s?page=%d&tab=repositories", target.Username, page)
			resp, err := client.Get(ctx, url)
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()

			doc, err := goquery.NewDocumentFromReader(resp.Body)
			if err != nil {
				return err
			}

			pageRepos := parseReposPage(doc, target.Username)
			mu.Lock()
			repos = append(repos, pageRepos...)
			_ = bar.Add(len(pageRepos))
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	_ = bar.Finish()
	return repos, nil
}

func parseReposPage(doc *goquery.Document, username string) []models.RepoDetails {
	var repos []models.RepoDetails
	doc.Find("#user-repositories-list li").Each(func(_ int, repo *goquery.Selection) {
		name := strings.TrimSpace(repo.Find(`a[itemprop="name codeRepository"]`).Text())
		if name == "" {
			return
		}

		lang := strings.TrimSpace(repo.Find(`span[itemprop="programmingLanguage"]`).Text())

		stars := 0
		if starEl := repo.Find(fmt.Sprintf(`a[href="/%s/%s/stargazers"]`, username, name)); starEl.Length() > 0 {
			stars = parseCount(starEl.Text())
		}

		forks := 0
		if forkEl := repo.Find(fmt.Sprintf(`a[href="/%s/%s/network/members"]`, username, name)); forkEl.Length() > 0 {
			forks = parseCount(forkEl.Text())
		}

		classes, _ := repo.Attr("class")
		classList := strings.Fields(classes)

		repos = append(repos, models.RepoDetails{
			Name:       name,
			Language:   lang,
			Stars:      stars,
			Forks:      forks,
			IsFork:     containsClass(classList, "fork"),
			IsMirror:   containsClass(classList, "mirror"),
			IsSource:   containsClass(classList, "source"),
			IsArchived: containsClass(classList, "archived"),
		})
	})
	return repos
}

func parseCount(s string) int {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	n, _ := strconv.Atoi(s)
	return n
}

func containsClass(classes []string, target string) bool {
	for _, c := range classes {
		if c == target {
			return true
		}
	}
	return false
}

// ShowRepos prints repository statistics.
func ShowRepos(target *models.Target) {
	if len(target.Repos) == 0 {
		fmt.Println("[-] No repositories found.")
		return
	}

	sourceCount := 0
	forkCount := 0
	for _, r := range target.Repos {
		if r.IsSource {
			sourceCount++
		}
		if r.IsFork {
			forkCount++
		}
	}

	fmt.Printf("[+] %d repositories scraped! (%d sources, %d forks)\n", len(target.Repos), sourceCount, forkCount)

	if len(target.LanguagesStats) > 0 {
		fmt.Println("\n[+] Languages stats:")
		count := 0
		for lang, pct := range target.LanguagesStats {
			if count >= 4 {
				break
			}
			fmt.Printf("- %s (%.2f%%)\n", lang, pct)
			count++
		}
	}
}

// ComputeLanguageStats calculates language percentages from repos.
func ComputeLanguageStats(repos []models.RepoDetails) map[string]float64 {
	if len(repos) == 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, r := range repos {
		if r.Language != "" {
			counts[r.Language]++
		}
	}
	stats := make(map[string]float64)
	total := float64(len(repos))
	for lang, count := range counts {
		stats[lang] = math.Round(float64(count)/total*10000) / 100
	}
	return stats
}
