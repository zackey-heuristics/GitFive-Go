package scraper

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"golang.org/x/sync/semaphore"
)

func TestScrapeCommits_MapsLinkedAccounts(t *testing.T) {
	emailsIndex := map[string]string{
		"sha-alice": "alice@example.com",
		"sha-bob":   "bob@example.com",
		"sha-noone": "ghost@example.com",
	}

	_, cleanup := withFakeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/alice/tmp/commits" && r.URL.Query().Get("page") == "1":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{
					"sha": "sha-alice",
					"commit": {"author": {"name": "alice", "email": "alice@example.com"}},
					"author": {"login": "AliceGH", "avatar_url": "https://x/a.png"}
				},
				{
					"sha": "sha-bob",
					"commit": {"author": {"name": "bob", "email": "bob@example.com"}},
					"author": {"login": "bob-gh", "avatar_url": "https://x/b.png"}
				},
				{
					"sha": "sha-noone",
					"commit": {"author": {"name": "ghost", "email": "ghost@example.com"}},
					"author": null
				}
			]`))
		case r.URL.Path == "/users/AliceGH":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"login":"AliceGH","name":"Alice Smith"}`))
		case r.URL.Path == "/users/bob-gh":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"login":"bob-gh","name":""}`))
		default:
			t.Errorf("unexpected request: %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer cleanup()

	sem := semaphore.NewWeighted(2)
	got, err := ScrapeCommits(context.Background(), "github_pat_xxxx", "alice", "tmp",
		emailsIndex, "AliceGH", false, sem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 mapped accounts, got %d: %v", len(got), got)
	}
	a, ok := got["alice@example.com"]
	if !ok {
		t.Fatalf("missing alice mapping; got %v", got)
	}
	if a.Username != "AliceGH" {
		t.Errorf("alice.Username = %q, want AliceGH", a.Username)
	}
	if !a.IsTarget {
		t.Errorf("alice.IsTarget = false, want true (case-insensitive match)")
	}
	if a.FullName != "Alice Smith" {
		t.Errorf("alice.FullName = %q, want Alice Smith", a.FullName)
	}
	b, ok := got["bob@example.com"]
	if !ok {
		t.Fatalf("missing bob mapping; got %v", got)
	}
	if b.IsTarget {
		t.Errorf("bob.IsTarget = true, want false")
	}
	if _, ok := got["ghost@example.com"]; ok {
		t.Errorf("ghost should not be mapped (author is null)")
	}
}

func TestScrapeCommits_CheckOnlySkipsProfileFetch(t *testing.T) {
	emailsIndex := map[string]string{"sha-alice": "alice@example.com"}

	_, cleanup := withFakeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/AliceGH" {
			t.Error("checkOnly=true should not trigger /users/{login} fetch")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{
				"sha": "sha-alice",
				"commit": {"author": {"name": "alice", "email": "alice@example.com"}},
				"author": {"login": "AliceGH"}
			}
		]`))
	}))
	defer cleanup()

	sem := semaphore.NewWeighted(1)
	got, err := ScrapeCommits(context.Background(), "github_pat_xxxx", "alice", "tmp",
		emailsIndex, "", true, sem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["alice@example.com"].FullName != "" {
		t.Errorf("FullName should be empty when checkOnly=true, got %q", got["alice@example.com"].FullName)
	}
}

func TestScrapeCommits_SkipsCommitWithMismatchedEmail(t *testing.T) {
	// Defense in depth: if the API returned a sha we have but the commit
	// body shows a different email, skip silently.
	emailsIndex := map[string]string{"sha-alice": "alice@example.com"}

	_, cleanup := withFakeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{
				"sha": "sha-alice",
				"commit": {"author": {"name": "x", "email": "tampered@example.com"}},
				"author": {"login": "AliceGH"}
			}
		]`))
	}))
	defer cleanup()

	sem := semaphore.NewWeighted(1)
	got, err := ScrapeCommits(context.Background(), "github_pat_xxxx", "alice", "tmp",
		emailsIndex, "", true, sem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("mismatched-email commit should be skipped, got %v", got)
	}
}

func TestScrapeCommits_EmptyEmailsIndexReturnsEmpty(t *testing.T) {
	// No HTTP server attached; this proves we short-circuit before any call.
	sem := semaphore.NewWeighted(1)
	got, err := ScrapeCommits(context.Background(), "github_pat_xxxx", "alice", "tmp",
		map[string]string{}, "", true, sem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

func TestScrapeCommits_PaginatesUntilShortPage(t *testing.T) {
	// 150 spoofed commits -> requires 2 pages at per_page=100.
	emailsIndex := make(map[string]string, 150)
	for i := 0; i < 150; i++ {
		emailsIndex[fmt.Sprintf("sha-%d", i)] = fmt.Sprintf("user%d@example.com", i)
	}

	var pageCalls int32

	_, cleanup := withFakeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/users/") {
			t.Errorf("unexpected /users fetch in pagination test: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		atomic.AddInt32(&pageCalls, 1)
		var page int
		_, _ = fmt.Sscanf(r.URL.Query().Get("page"), "%d", &page)

		var b strings.Builder
		b.WriteString("[")
		switch page {
		case 1:
			for i := 0; i < 100; i++ {
				if i > 0 {
					b.WriteString(",")
				}
				_, _ = fmt.Fprintf(&b, `{"sha":"sha-%d","commit":{"author":{"name":"u","email":"user%d@example.com"}},"author":{"login":"login%d"}}`, i, i, i)
			}
		case 2:
			for i := 100; i < 150; i++ {
				if i > 100 {
					b.WriteString(",")
				}
				_, _ = fmt.Fprintf(&b, `{"sha":"sha-%d","commit":{"author":{"name":"u","email":"user%d@example.com"}},"author":{"login":"login%d"}}`, i, i, i)
			}
		}
		b.WriteString("]")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(b.String()))
	}))
	defer cleanup()

	sem := semaphore.NewWeighted(1)
	got, err := ScrapeCommits(context.Background(), "github_pat_xxxx", "alice", "tmp",
		emailsIndex, "", true, sem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 150 {
		t.Errorf("expected 150 mappings, got %d", len(got))
	}
	if pageCalls != 2 {
		t.Errorf("expected 2 page fetches, got %d", pageCalls)
	}
}

func TestScrapeCommits_ApiErrorPropagated(t *testing.T) {
	emailsIndex := map[string]string{"sha-x": "x@example.com"}

	_, cleanup := withFakeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Resource not accessible by personal access token"}`))
	}))
	defer cleanup()

	sem := semaphore.NewWeighted(1)
	_, err := ScrapeCommits(context.Background(), "github_pat_xxxx", "alice", "tmp",
		emailsIndex, "", true, sem)
	if err == nil {
		t.Fatal("expected error on 403, got nil")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error %q should contain 403", err.Error())
	}
}
