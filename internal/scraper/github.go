package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/zackey-heuristics/gitfive-go/internal/httpclient"
)

// CreateRepo creates a private GitHub repository via the web interface.
func CreateRepo(ctx context.Context, client *httpclient.Client, owner, repoName string) error {
	payload := map[string]interface{}{
		"owner":                   owner,
		"template_repository_id":  "",
		"include_all_branches":    "0",
		"repository": map[string]string{
			"name":               repoName,
			"visibility":         "private",
			"description":        "",
			"auto_init":          "0",
			"license_template":   "",
			"gitignore_template": "",
		},
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/repositories", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Github-Verified-Fetch", "true")
	req.Header.Set("Origin", "https://github.com")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 302 {
		return nil
	}
	respBody, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("couldn't create repo %q (status %d): %s", repoName, resp.StatusCode, string(respBody))
}

// DeleteRepo deletes a GitHub repository via the web settings form.
func DeleteRepo(ctx context.Context, client *httpclient.Client, username, repoName, password string) error {
	resp, err := client.Get(ctx, fmt.Sprintf("https://github.com/%s/%s/settings", username, repoName))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return err
	}

	action := fmt.Sprintf("/%s/%s/settings/delete", username, repoName)
	form := doc.Find(fmt.Sprintf(`form[action="%s"]`, action))
	token, _ := form.Find(`input[name="authenticity_token"]`).Attr("value")
	if token == "" {
		return fmt.Errorf("couldn't find delete form for repo %q", repoName)
	}

	formData := url.Values{
		"_method":              {"delete"},
		"authenticity_token":   {token},
		"repository":          {repoName},
		"sudo_referrer":       {fmt.Sprintf("https://github.com/%s/%s/settings", username, repoName)},
		"user_id":             {username},
		"verify":              {fmt.Sprintf("%s/%s", username, repoName)},
		"sudo_password":       {password},
	}

	delResp, err := client.PostForm(ctx, fmt.Sprintf("https://github.com/%s/%s/settings/delete", username, repoName), formData)
	if err != nil {
		return err
	}
	defer delResp.Body.Close()

	if delResp.StatusCode == 200 || delResp.StatusCode == 302 {
		return nil
	}
	return fmt.Errorf("couldn't delete repo %q (status %d)", repoName, delResp.StatusCode)
}

// FetchProfileName uses GitHub hovercards to fetch the display name for a username.
func FetchProfileName(ctx context.Context, client *httpclient.Client, username string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://github.com/users/%s/hovercard", username), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	var fields []string
	doc.Find(`section[aria-label="User login and name"] a`).Each(func(_ int, s *goquery.Selection) {
		fields = append(fields, strings.TrimSpace(s.Text()))
	})

	if len(fields) < 2 {
		return "", nil
	}
	return fields[1], nil
}

// GetOriginalBranchFromCommit finds the branch a commit belongs to.
func GetOriginalBranchFromCommit(ctx context.Context, client *httpclient.Client, username, repoName, commit string) (string, error) {
	resp, err := client.Get(ctx, fmt.Sprintf("https://github.com/%s/%s/branch_commits/%s", username, repoName, commit))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	branch := strings.TrimSpace(doc.Find("li.branch").Text())
	if branch == "" {
		return "", fmt.Errorf("branch not found for %s/%s commit %s", username, repoName, commit)
	}
	return branch, nil
}

var blobURLRegex = regexp.MustCompile(`github\.com/(.*?)/(.*?)/blob/(.{40})/(.*?)$`)

// ParseBlobURL extracts username, repo, commit, and filename from a GitHub blob URL.
func ParseBlobURL(blobURL string) (username, repoName, commit, filename string, err error) {
	matches := blobURLRegex.FindStringSubmatch(blobURL)
	if len(matches) < 5 {
		return "", "", "", "", fmt.Errorf("cannot parse blob URL: %s", blobURL)
	}
	return matches[1], matches[2], matches[3], matches[4], nil
}
