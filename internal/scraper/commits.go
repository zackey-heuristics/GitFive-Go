package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// CommitAccount holds the result of commit-based email discovery.
type CommitAccount struct {
	Avatar   string `json:"avatar"`
	FullName string `json:"full_name,omitempty"`
	Username string `json:"username"`
	IsTarget bool   `json:"is_target"`
}

// commitsAPIPerPage is GitHub's max page size for the commits endpoint. Using
// the cap keeps round-trip count to ceil(N/100) for N spoofed commits.
const commitsAPIPerPage = 100

// commitsAPIPageCap bounds the worst-case loop. Each page brings 100 commits;
// 50 pages = 5000 commits. The metamon flow generates far fewer than that.
const commitsAPIPageCap = 50

// apiCommit mirrors the subset of `GET /repos/{owner}/{repo}/commits` we use.
// `Author` is the top-level resolved GitHub user (null when GitHub could not
// link the email to an account); `Commit.Author.Email` is the raw email we
// pushed — it identifies which spoofed commit this is via the sha map.
type apiCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Author struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"commit"`
	Author *apiCommitUser `json:"author"`
}

type apiCommitUser struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

// ScrapeCommits walks `GET /repos/{owner}/{repo}/commits?sha=mirage` and maps
// each spoofed commit's email to the GitHub account GitHub linked it to via
// the top-level `author.login` field. Token is the fine-grained PAT.
func ScrapeCommits(ctx context.Context, token, owner, repoName string,
	emailsIndex map[string]string, targetUsername string, checkOnly bool, sem Semaphore) (map[string]*CommitAccount, error) {

	out := make(map[string]*CommitAccount)

	if len(emailsIndex) == 0 {
		return out, nil
	}

	// Walk pages serially. Pagination is small (1-10 pages typically) and
	// serial keeps the implementation simple; the per-page work is small.
	// `out` therefore needs no mutex — only this goroutine writes to it.
	for page := 1; page <= commitsAPIPageCap; page++ {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		url := fmt.Sprintf("%s/repos/%s/%s/commits?sha=mirage&per_page=%d&page=%d",
			githubAPIBase, owner, repoName, commitsAPIPerPage, page)

		commits, err := fetchCommitsPage(ctx, token, url)
		if err != nil {
			return nil, err
		}
		if len(commits) == 0 {
			break
		}

		if err := processAPICommits(ctx, token, commits, emailsIndex, targetUsername, checkOnly, sem, out); err != nil {
			return out, err
		}

		if len(commits) < commitsAPIPerPage {
			break
		}
	}

	return out, nil
}

func fetchCommitsPage(ctx context.Context, token, url string) ([]apiCommit, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	setGitHubAPIHeaders(req, token)

	resp, err := apiHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("commits page fetch failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusConflict {
		// 409 = "Git Repository is empty." after a fresh CreateRepo before push.
		return nil, fmt.Errorf("empty repository")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, fmt.Errorf("commits API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Cap the JSON read; one page at per_page=100 is comfortably under 1 MiB.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read commits page: %w", err)
	}
	var commits []apiCommit
	if err := json.Unmarshal(body, &commits); err != nil {
		return nil, fmt.Errorf("parse commits page: %w", err)
	}
	return commits, nil
}

func processAPICommits(ctx context.Context, token string,
	commits []apiCommit, emailsIndex map[string]string,
	targetUsername string, checkOnly bool, sem Semaphore,
	out map[string]*CommitAccount) error {

	for _, c := range commits {
		if err := ctx.Err(); err != nil {
			return err
		}
		// We only care about commits we pushed. The init commit (no sha in
		// the index) and any noise are skipped here implicitly.
		email, ok := emailsIndex[c.SHA]
		if !ok {
			continue
		}
		// Sanity: the spoofed-commit author email should match what we
		// recorded for that sha. If not, skip (defense in depth).
		if !strings.EqualFold(c.Commit.Author.Email, email) {
			continue
		}
		// `author` (top-level) is null when GitHub couldn't resolve the
		// email to an account — that's the "no match" outcome we silently
		// skip, mirroring the old HTML behaviour.
		if c.Author == nil || c.Author.Login == "" {
			continue
		}

		account := &CommitAccount{
			Avatar:   c.Author.AvatarURL,
			Username: c.Author.Login,
			IsTarget: strings.EqualFold(c.Author.Login, targetUsername),
		}

		if account.IsTarget {
			fmt.Printf("[+] [Target's email] %s -> @%s\n", email, c.Author.Login)
		}

		if !checkOnly {
			// Bound concurrency on the profile-name lookup the same way
			// the HTML version did. Acquire failure here means ctx was
			// cancelled — propagate it instead of silently skipping.
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			name, _ := FetchProfileName(ctx, token, c.Author.Login)
			sem.Release(1)
			account.FullName = name
		}

		out[email] = account
	}
	return nil
}
