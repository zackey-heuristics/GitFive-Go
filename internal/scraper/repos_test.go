package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/sync/semaphore"

	"github.com/zackey-heuristics/gitfive-go/internal/httpclient"
	"github.com/zackey-heuristics/gitfive-go/internal/models"
)

// Note: FetchReposList requires URL injection to test properly.
// parseReposPage is tested directly below as it contains the core logic.

const reposHTML = `<html><body>
<div id="user-repositories-list">
  <li class="source public">
    <a itemprop="name codeRepository">my-repo</a>
    <span itemprop="programmingLanguage">Go</span>
    <a href="/testuser/my-repo/stargazers"> 42 </a>
    <a href="/testuser/my-repo/network/members"> 5 </a>
  </li>
  <li class="fork public">
    <a itemprop="name codeRepository">forked-repo</a>
    <span itemprop="programmingLanguage">Python</span>
  </li>
</ul>
</div>
</body></html>`

func TestParseReposPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, reposHTML)
	}))
	defer srv.Close()

	client := httpclient.New()
	resp, err := client.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	doc, _ := goquery.NewDocumentFromReader(resp.Body)
	repos := parseReposPage(doc, "testuser")

	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}

	if repos[0].Name != "my-repo" {
		t.Errorf("expected 'my-repo', got %q", repos[0].Name)
	}
	if repos[0].Language != "Go" {
		t.Errorf("expected language 'Go', got %q", repos[0].Language)
	}
	if repos[0].Stars != 42 {
		t.Errorf("expected 42 stars, got %d", repos[0].Stars)
	}
	if repos[0].Forks != 5 {
		t.Errorf("expected 5 forks, got %d", repos[0].Forks)
	}
	if !repos[0].IsSource {
		t.Error("expected my-repo to be source")
	}
	if repos[0].IsFork {
		t.Error("expected my-repo to not be fork")
	}

	if repos[1].Name != "forked-repo" {
		t.Errorf("expected 'forked-repo', got %q", repos[1].Name)
	}
	if !repos[1].IsFork {
		t.Error("expected forked-repo to be fork")
	}
}

func TestComputeLanguageStats(t *testing.T) {
	repos := []models.RepoDetails{
		{Name: "a", Language: "Go"},
		{Name: "b", Language: "Go"},
		{Name: "c", Language: "Python"},
		{Name: "d", Language: ""},
	}
	stats := ComputeLanguageStats(repos)
	if stats["Go"] != 50.0 {
		t.Errorf("expected Go=50%%, got %.2f%%", stats["Go"])
	}
	if stats["Python"] != 25.0 {
		t.Errorf("expected Python=25%%, got %.2f%%", stats["Python"])
	}
}

func TestParseCount(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"42", 42},
		{" 1,234 ", 1234},
		{"0", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := parseCount(tt.input)
		if got != tt.want {
			t.Errorf("parseCount(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// Ensure Semaphore interface works with real semaphore
func TestSemaphoreInterface(t *testing.T) {
	var sem Semaphore = semaphore.NewWeighted(5)
	if err := sem.Acquire(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	sem.Release(1)
}
