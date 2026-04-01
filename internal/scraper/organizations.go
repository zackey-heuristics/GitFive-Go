package scraper

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/sync/errgroup"

	"github.com/zackey-heuristics/gitfive-go/internal/httpclient"
	"github.com/zackey-heuristics/gitfive-go/internal/ui"
	"github.com/zackey-heuristics/gitfive-go/internal/util"
)

// OrgInfo holds scraped organization data.
type OrgInfo struct {
	Handle            string            `json:"handle"`
	Name              string            `json:"name,omitempty"`
	Email             string            `json:"email,omitempty"`
	WebsiteDomains    []string          `json:"website_domains,omitempty"`
	Website           WebsiteInfo       `json:"website"`
	WebsiteOnMainRepo WebsiteInfo       `json:"website_on_main_repo"`
	GitHubPages       GitHubPagesInfo   `json:"github_pages"`
}

// WebsiteInfo holds website scraping results.
type WebsiteInfo struct {
	Link           string `json:"link,omitempty"`
	GHPagesHosted  bool   `json:"ghpages_hosted"`
}

// GitHubPagesInfo holds GitHub Pages detection results.
type GitHubPagesInfo struct {
	Activated bool   `json:"activated"`
	Link      string `json:"link,omitempty"`
	CNAME     string `json:"cname,omitempty"`
}

// ScrapeOrgs fetches all organizations for a user.
func ScrapeOrgs(ctx context.Context, client *httpclient.Client, username string, sem Semaphore) ([]OrgInfo, error) {
	resp, err := client.Get(ctx, fmt.Sprintf("https://github.com/%s", username))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var orgNames []string
	doc.Find(`a.avatar-group-item[data-hovercard-type="organization"]`).Each(func(_ int, s *goquery.Selection) {
		if _, exists := s.Attr("itemprop"); exists {
			if label, exists := s.Attr("aria-label"); exists && label != "" {
				orgNames = append(orgNames, label)
			}
		}
	})

	var mu sync.Mutex
	var orgs []OrgInfo

	bar := ui.NewProgressBar(len(orgNames), "Fetching organizations...")

	g, ctx := errgroup.WithContext(ctx)
	for _, orgName := range orgNames {
		orgName := orgName
		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			org, err := fetchOrg(ctx, client, orgName)
			if err != nil {
				return err
			}

			mu.Lock()
			orgs = append(orgs, org)
			mu.Unlock()
			bar.Add(1)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	bar.Finish()
	return orgs, nil
}

func fetchOrg(ctx context.Context, client *httpclient.Client, orgName string) (OrgInfo, error) {
	org := OrgInfo{Handle: orgName}

	resp, err := client.Get(ctx, fmt.Sprintf("https://github.com/%s", orgName))
	if err != nil {
		return org, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return org, err
	}

	org.Name = strings.TrimSpace(doc.Find("h1").Text())

	websiteLink := strings.TrimSpace(doc.Find(`a[itemprop="url"]`).Text())
	org.WebsiteDomains = util.DetectCustomDomain(websiteLink)
	ghPagesHosted := false
	if len(org.WebsiteDomains) > 0 {
		ghPagesHosted = util.IsGHPagesHosted(org.WebsiteDomains[len(org.WebsiteDomains)-1])
	}
	org.Website = WebsiteInfo{Link: websiteLink, GHPagesHosted: ghPagesHosted}

	org.Email = strings.TrimSpace(doc.Find(`a[itemprop="email"]`).Text())

	// Check org/org repo for website
	resp1, err := client.Get(ctx, fmt.Sprintf("https://github.com/%s/%s", orgName, orgName))
	if err != nil {
		return org, err
	}
	defer resp1.Body.Close()
	doc1, _ := goquery.NewDocumentFromReader(resp1.Body)

	repoWebsiteLink := strings.TrimSpace(doc1.Find(`a[role="link"]`).Text())
	repoWebsiteDomains := util.DetectCustomDomain(websiteLink)
	repoGHPagesHosted := false
	if len(repoWebsiteDomains) > 0 {
		repoGHPagesHosted = util.IsGHPagesHosted(repoWebsiteDomains[len(repoWebsiteDomains)-1])
	}
	org.WebsiteOnMainRepo = WebsiteInfo{Link: repoWebsiteLink, GHPagesHosted: repoGHPagesHosted}

	// GitHub Pages check
	resp2, err := client.Get(ctx, fmt.Sprintf("https://github.com/%s/%s.github.io", orgName, orgName))
	if err != nil {
		return org, err
	}
	defer resp2.Body.Close()
	doc2, _ := goquery.NewDocumentFromReader(resp2.Body)

	ghPages := GitHubPagesInfo{}
	if checkGitHubPages(ctx, client, &ghPages, resp1.StatusCode, doc1, orgName, orgName) ||
		checkGitHubPages(ctx, client, &ghPages, resp2.StatusCode, doc2, orgName+".github.io", orgName) {
	}
	org.GitHubPages = ghPages

	return org, nil
}

var defaultBranchRegex = regexp.MustCompile(`,"defaultBranch":"(.*?)","`)

func checkGitHubPages(ctx context.Context, client *httpclient.Client, ghPages *GitHubPagesInfo, statusCode int, doc *goquery.Document, repoName, orgName string) bool {
	if statusCode != 200 {
		return false
	}

	if repoName != orgName {
		ghPages.Activated = true
		ghPages.Link = repoName
	}

	// Check for CNAME file
	bodyHTML, _ := doc.Html()
	matches := defaultBranchRegex.FindStringSubmatch(bodyHTML)
	if len(matches) < 2 {
		return ghPages.Activated
	}

	cnameURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/CNAME", orgName, repoName, matches[1])
	cnameResp, err := client.Get(ctx, cnameURL)
	if err != nil || cnameResp.StatusCode != 200 {
		if cnameResp != nil {
			cnameResp.Body.Close()
		}
		return ghPages.Activated
	}
	defer cnameResp.Body.Close()

	domain := strings.TrimSpace(ReadAll(cnameResp))

	domains := util.DetectCustomDomain(domain)
	if len(domains) > 0 {
		ghPages.CNAME = domains[len(domains)-1]
		if repoName == orgName {
			ghPages.Activated = true
			ghPages.Link = domains[len(domains)-1]
		}
	}

	return ghPages.Activated
}

// ShowOrgs prints organization information.
func ShowOrgs(orgs []OrgInfo) {
	if len(orgs) == 0 {
		fmt.Println("[-] No organizations found.")
		return
	}

	fmt.Printf("[+] %d organization(s) found!\n\n", len(orgs))

	for i, org := range orgs {
		fmt.Printf("Handle: %s\n", org.Handle)
		if org.Name != "" {
			fmt.Printf("Name: %s\n", org.Name)
		}
		if org.Website.Link != "" {
			hosted := ""
			if org.Website.GHPagesHosted {
				hosted = " (Hosted on GitHub Pages!)"
			}
			fmt.Printf("Website: %s%s\n", org.Website.Link, hosted)
		}
		if org.WebsiteOnMainRepo.Link != "" {
			hosted := ""
			if org.WebsiteOnMainRepo.GHPagesHosted {
				hosted = " (Hosted on GitHub Pages!)"
			}
			fmt.Printf("Website on main repo: %s%s\n", org.WebsiteOnMainRepo.Link, hosted)
		}
		if org.Email != "" {
			fmt.Printf("Email: %s\n", org.Email)
		}
		status := "Not found"
		if org.GitHubPages.Activated {
			status = "Found"
		}
		fmt.Printf("GH Pages: %s\n", status)
		if org.GitHubPages.Link != "" {
			fmt.Printf("GH Pages link: %s\n", org.GitHubPages.Link)
		}
		if org.GitHubPages.CNAME != "" {
			fmt.Printf("GH Pages CNAME: %s\n", org.GitHubPages.CNAME)
		}
		if i < len(orgs)-1 {
			fmt.Println()
		}
	}
}
