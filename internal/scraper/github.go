package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/zackey-heuristics/gitfive-go/internal/httpclient"
)

// githubAPIBase is the REST API endpoint. Tests may override this, but only
// before any goroutine using CreateRepo/DeleteRepo runs (i.e. in TestMain or
// at the top of a non-parallel test).
var githubAPIBase = "https://api.github.com"

// apiHTTPClient enforces an explicit timeout because http.DefaultClient has
// none; combined with request-context cancellation it bounds worst-case wait
// even for misbehaving connections.
var apiHTTPClient = &http.Client{Timeout: 60 * time.Second}

// errBadIdent is returned when an owner/repo name contains URL-significant
// characters that would change which endpoint we actually hit.
var errBadIdent = fmt.Errorf("owner or repo name contains forbidden characters")

func setGitHubAPIHeaders(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

// validateRepoIdent rejects names that would alter the URL path (slash) or
// inject query/fragment components. GitHub's own naming rules forbid these
// already, but the check defends against accidental misuse from callers.
func validateRepoIdent(s string) error {
	if s == "" {
		return errBadIdent
	}
	if strings.ContainsAny(s, "/?#") {
		return errBadIdent
	}
	return nil
}

// maxErrorBodyBytes caps how much of an API error response we read into
// memory and surface in the error message.
const maxErrorBodyBytes = 4096

// CreateRepo creates a private repository owned by the authenticated user via
// `POST /user/repos`. Requires a fine-grained PAT with Administration: Write
// on All repositories.
func CreateRepo(ctx context.Context, token, repoName string) error {
	if err := validateRepoIdent(repoName); err != nil {
		return fmt.Errorf("create repo %q: %w", repoName, err)
	}
	payload, _ := json.Marshal(map[string]any{
		"name":      repoName,
		"private":   true,
		"auto_init": false,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", githubAPIBase+"/user/repos", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	setGitHubAPIHeaders(req, token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := apiHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("create repo %q: %w", repoName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusCreated {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	return fmt.Errorf("couldn't create repo %q (status %d): %s", repoName, resp.StatusCode, string(body))
}

// DeleteRepo deletes a repository via `DELETE /repos/{owner}/{repo}`. Requires
// a fine-grained PAT with Administration: Write on the target repo.
func DeleteRepo(ctx context.Context, token, owner, repoName string) error {
	if err := validateRepoIdent(owner); err != nil {
		return fmt.Errorf("delete repo %q: %w", repoName, err)
	}
	if err := validateRepoIdent(repoName); err != nil {
		return fmt.Errorf("delete repo %q: %w", repoName, err)
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s", githubAPIBase, owner, repoName)
	req, err := http.NewRequestWithContext(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	setGitHubAPIHeaders(req, token)

	resp, err := apiHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete repo %q: %w", repoName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	return fmt.Errorf("couldn't delete repo %q (status %d): %s", repoName, resp.StatusCode, string(body))
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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

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
