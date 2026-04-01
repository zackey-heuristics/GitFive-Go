package scraper

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/sync/errgroup"

	"github.com/zackey-heuristics/gitfive-go/internal/httpclient"
	"github.com/zackey-heuristics/gitfive-go/internal/models"
)

// GetFollows returns the set of usernames from the target's followers or following list.
// toScrape should be "followers" or "following".
func GetFollows(ctx context.Context, client *httpclient.Client, username, toScrape string, sem Semaphore) (models.StringSet, error) {
	resp, err := client.Get(ctx, fmt.Sprintf("https://github.com/%s?tab=%s", username, toScrape))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse follower/following count
	countText := ""
	doc.Find(fmt.Sprintf(`a[href="/%s?tab=%s"]`, username, toScrape)).Each(func(_ int, s *goquery.Selection) {
		countText = s.Text()
	})

	count := parseFollowCount(countText)
	if count == 0 {
		return models.NewStringSet(), nil
	}

	// Calculate pages (50 per page), ceiling division
	numPages := (count + 49) / 50

	var mu sync.Mutex
	usernames := models.NewStringSet()

	// Parse first page
	extractUsernames(doc, &mu, usernames)

	if numPages > 1 {
		g, ctx := errgroup.WithContext(ctx)
		for page := 2; page <= numPages; page++ {
			page := page
			g.Go(func() error {
				if err := sem.Acquire(ctx, 1); err != nil {
					return err
				}
				defer sem.Release(1)

				url := fmt.Sprintf("https://github.com/%s?page=%d&tab=%s", username, page, toScrape)
				resp, err := client.Get(ctx, url)
				if err != nil {
					return err
				}
				defer resp.Body.Close()

				pageDoc, err := goquery.NewDocumentFromReader(resp.Body)
				if err != nil {
					return err
				}

				extractUsernames(pageDoc, &mu, usernames)
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
	}

	return usernames, nil
}

func extractUsernames(doc *goquery.Document, mu *sync.Mutex, usernames models.StringSet) {
	doc.Find(`a[data-hovercard-type="user"]`).Each(func(_ int, s *goquery.Selection) {
		if s.Find("span").Length() > 0 {
			href, exists := s.Attr("href")
			if exists {
				username := strings.Trim(href, "/")
				mu.Lock()
				usernames.Add(username)
				mu.Unlock()
			}
		}
	})
}

func parseFollowCount(text string) int {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "k", "00")
	text = strings.ReplaceAll(text, ".", "")
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return 0
	}
	return parseCount(parts[0])
}
