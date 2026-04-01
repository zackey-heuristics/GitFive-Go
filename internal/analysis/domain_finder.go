package analysis

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/zackey-heuristics/gitfive-go/internal/models"
	"github.com/zackey-heuristics/gitfive-go/internal/util"
)

// GuessCustomDomain searches Google for the company domain from the target's profile.
func GuessCustomDomain(target *models.Target) models.StringSet {
	company := strings.ToLower(target.Company)
	if company == "" {
		return models.NewStringSet()
	}

	// Simple Google search via scraping
	domain := searchGoogle(company)
	if domain != "" {
		fmt.Printf("[Google] Found possible domain %q for company %q\n", domain, company)
		result := models.NewStringSet()
		result.Add(domain)
		return result
	}
	return models.NewStringSet()
}

func searchGoogle(query string) string {
	if strings.EqualFold(query, "google") {
		return "google.com"
	}

	searchURL := fmt.Sprintf("https://www.google.com/search?q=%s&num=5", strings.ReplaceAll(query, " ", "+"))
	resp, err := http.Get(searchURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return ""
	}

	socialDomains := []string{"facebook.com", "twitter.com", "linkedin.com", "instagram.com", "youtube.com"}

	var result string
	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		if result != "" {
			return
		}
		href, exists := s.Attr("href")
		if !exists || !strings.Contains(href, "http") {
			return
		}

		for _, social := range socialDomains {
			if strings.Contains(href, social) && !strings.Contains(strings.ToLower(query), strings.TrimSuffix(social, ".com")) {
				return
			}
		}

		domain := util.ExtractDomain(href, 0)
		if domain != "" && !strings.Contains(domain, "google") {
			result = domain
		}
	})

	return result
}
