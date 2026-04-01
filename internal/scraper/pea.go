package scraper

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/sync/errgroup"

	"github.com/zackey-heuristics/gitfive-go/internal/httpclient"
	"github.com/zackey-heuristics/gitfive-go/internal/models"
	"github.com/zackey-heuristics/gitfive-go/internal/ui"
)

// AnalyzePEA checks each username for "Pretty Empty Account" status.
// Returns a map of username -> isPEA.
func AnalyzePEA(ctx context.Context, client *httpclient.Client, usernames models.StringSet, sem Semaphore) (map[string]bool, error) {
	users := make(map[string]bool)
	var mu sync.Mutex

	list := usernames.Slice()

	if len(list) == 0 {
		return users, nil
	}

	bar := ui.NewProgressBar(len(list), "Analyzing following & followers...")

	g, ctx := errgroup.WithContext(ctx)
	for _, u := range list {
		u := u
		g.Go(func() error {
			pea, err := IsPEA(ctx, client, u, sem)
			if err != nil {
				return err
			}
			mu.Lock()
			users[u] = pea
			mu.Unlock()
			_ = bar.Add(1)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	_ = bar.Finish()
	return users, nil
}

// IsPEA checks if a GitHub account is a "Pretty Empty Account".
func IsPEA(ctx context.Context, client *httpclient.Client, username string, sem Semaphore) (bool, error) {
	hasManyStars, err := HasManyStarsOrRepos(ctx, client, username, sem)
	if err != nil {
		return false, err
	}
	if hasManyStars {
		return false, nil
	}

	isFollowed, err := IsFollowedOrFollowingALot(ctx, client, username, sem)
	if err != nil {
		return false, err
	}
	return !isFollowed, nil
}

// IsFollowedOrFollowingALot checks if user has >= 20 followers or >= 50 following.
func IsFollowedOrFollowingALot(ctx context.Context, client *httpclient.Client, username string, sem Semaphore) (bool, error) {
	if err := sem.Acquire(ctx, 1); err != nil {
		return false, err
	}
	defer sem.Release(1)

	resp, err := client.Get(ctx, fmt.Sprintf("https://github.com/%s", username))
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return false, err
	}

	followers := parseFollowCount(doc.Find(fmt.Sprintf(`a[href="/%s?tab=followers"]`, username)).Text())
	following := parseFollowCount(doc.Find(fmt.Sprintf(`a[href="/%s?tab=following"]`, username)).Text())

	return followers >= 20 || following >= 50, nil
}

// HasManyStarsOrRepos checks if user has repos with meaningful stars.
func HasManyStarsOrRepos(ctx context.Context, client *httpclient.Client, username string, sem Semaphore) (bool, error) {
	if err := sem.Acquire(ctx, 1); err != nil {
		return false, err
	}
	defer sem.Release(1)

	// Check for repos with > 0 stars
	resp, err := client.Get(ctx, fmt.Sprintf("https://github.com/%s?language=&q=stars:>0&tab=repositories", username))
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return false, err
	}

	if doc.Find(`a[itemprop="name codeRepository"]`).Length() == 0 {
		return false, nil
	}

	// Check for repos with >= 3 stars
	resp2, err := client.Get(ctx, fmt.Sprintf("https://github.com/%s?language=&q=stars:>=3&tab=repositories", username))
	if err != nil {
		return false, err
	}
	defer func() { _ = resp2.Body.Close() }()

	doc2, err := goquery.NewDocumentFromReader(resp2.Body)
	if err != nil {
		return false, err
	}

	if doc2.Find(`a[itemprop="name codeRepository"]`).Length() > 0 {
		return true, nil
	}

	// Fallback: extract stargazers
	stargazers, err := launchRepoQueries(ctx, client, username, doc, sem)
	if err != nil {
		return false, err
	}
	return len(stargazers) >= 2, nil
}

func launchRepoQueries(ctx context.Context, client *httpclient.Client, username string, doc *goquery.Document, sem Semaphore) (models.StringSet, error) {
	stargazers := models.NewStringSet()
	var mu sync.Mutex

	// Count repos
	totalText := strings.TrimSpace(doc.Find("div.user-repo-search-results-summary").Text())
	total := parseCount(strings.Fields(strings.ReplaceAll(totalText, "\n", ""))[0])
	if total == 0 {
		return stargazers, nil
	}

	pages := make([]int, 0)
	for i := 0; i < total && i/30 < 33; i += 30 {
		pages = append(pages, i)
	}

	g, ctx := errgroup.WithContext(ctx)
	for _, page := range pages {
		page := page
		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			mu.Lock()
			if len(stargazers) >= 2 {
				mu.Unlock()
				return nil
			}
			mu.Unlock()

			var pageDoc *goquery.Document
			if page == 0 {
				pageDoc = doc
			} else {
				cursor := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("cursor:%d", page)))
				resp, err := client.Get(ctx, fmt.Sprintf("https://github.com/%s?after=%s&language=&q=stars:>0&tab=repositories", username, url.QueryEscape(cursor)))
				if err != nil {
					return err
				}
				defer func() { _ = resp.Body.Close() }()
				pageDoc, err = goquery.NewDocumentFromReader(resp.Body)
				if err != nil {
					return err
				}
			}

			pageDoc.Find("#user-repositories-list li").Each(func(_ int, repo *goquery.Selection) {
				name := strings.TrimSpace(repo.Find(`a[itemprop="name codeRepository"]`).Text())
				if name == "" {
					return
				}
				sgs, _ := extractFirstStargazers(ctx, client, username, name)
				mu.Lock()
				for sg := range sgs {
					stargazers.Add(sg)
				}
				mu.Unlock()
			})
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return stargazers, nil
}

func extractFirstStargazers(ctx context.Context, client *httpclient.Client, username, repoName string) (models.StringSet, error) {
	stargazers := models.NewStringSet()
	resp, err := client.Get(ctx, fmt.Sprintf("https://github.com/%s/%s/stargazers", username, repoName))
	if err != nil {
		return stargazers, err
	}
	defer func() { _ = resp.Body.Close() }()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return stargazers, err
	}

	doc.Find("li.follow-list-item").Each(func(_ int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Find("h3.follow-list-name").Text())
		if !strings.EqualFold(name, username) {
			stargazers.Add(strings.ToLower(name))
		}
	})
	return stargazers, nil
}
