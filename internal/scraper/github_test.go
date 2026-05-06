package scraper

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseBlobURL(t *testing.T) {
	tests := []struct {
		url                                      string
		wantUser, wantRepo, wantCommit, wantFile string
		wantErr                                  bool
	}{
		{
			"https://github.com/alice/myrepo/blob/abc123def456abc123def456abc123def456abcd/path/to/file.go",
			"alice", "myrepo", "abc123def456abc123def456abc123def456abcd", "path/to/file.go",
			false,
		},
		{
			"not-a-blob-url",
			"", "", "", "",
			true,
		},
	}
	for _, tt := range tests {
		user, repo, commit, file, err := ParseBlobURL(tt.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseBlobURL(%q): err=%v, wantErr=%v", tt.url, err, tt.wantErr)
			continue
		}
		if user != tt.wantUser || repo != tt.wantRepo || commit != tt.wantCommit || file != tt.wantFile {
			t.Errorf("ParseBlobURL(%q) = (%q,%q,%q,%q), want (%q,%q,%q,%q)",
				tt.url, user, repo, commit, file,
				tt.wantUser, tt.wantRepo, tt.wantCommit, tt.wantFile)
		}
	}
}

// withFakeAPI replaces githubAPIBase with a test server URL for the duration
// of the test. Tests using this helper MUST NOT call t.Parallel().
func withFakeAPI(t *testing.T, h http.Handler) (string, func()) {
	t.Helper()
	srv := httptest.NewServer(h)
	prev := githubAPIBase
	githubAPIBase = srv.URL
	return srv.URL, func() {
		githubAPIBase = prev
		srv.Close()
	}
}

func TestCreateRepo(t *testing.T) {
	t.Run("success on 201", func(t *testing.T) {
		var gotMethod, gotPath, gotAuth, gotAPIVer string
		var gotBody []byte
		_, cleanup := withFakeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			gotAuth = r.Header.Get("Authorization")
			gotAPIVer = r.Header.Get("X-GitHub-Api-Version")
			gotBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"name":"x"}`))
		}))
		defer cleanup()

		if err := CreateRepo(context.Background(), "github_pat_xxxx", "myrepo"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotMethod != "POST" {
			t.Errorf("method = %q, want POST", gotMethod)
		}
		if gotPath != "/user/repos" {
			t.Errorf("path = %q, want /user/repos", gotPath)
		}
		if gotAuth != "Bearer github_pat_xxxx" {
			t.Errorf("auth = %q, want Bearer github_pat_xxxx", gotAuth)
		}
		if gotAPIVer != "2022-11-28" {
			t.Errorf("api version = %q, want 2022-11-28", gotAPIVer)
		}
		var payload map[string]any
		if err := json.Unmarshal(gotBody, &payload); err != nil {
			t.Fatalf("body not JSON: %v", err)
		}
		if payload["name"] != "myrepo" {
			t.Errorf("payload.name = %v, want myrepo", payload["name"])
		}
		if payload["private"] != true {
			t.Errorf("payload.private = %v, want true", payload["private"])
		}
	})

	t.Run("error on 422 includes status and body", func(t *testing.T) {
		_, cleanup := withFakeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"name already exists"}`))
		}))
		defer cleanup()

		err := CreateRepo(context.Background(), "github_pat_xxxx", "dup")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "422") || !strings.Contains(err.Error(), "name already exists") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects forbidden chars in repo name", func(t *testing.T) {
		// No HTTP server: the validation must fire before any network call.
		err := CreateRepo(context.Background(), "github_pat_xxxx", "evil/../../boom")
		if err == nil {
			t.Fatal("expected error for slash in repo name")
		}
		if !strings.Contains(err.Error(), "forbidden characters") {
			t.Errorf("error %q does not mention forbidden characters", err.Error())
		}
	})

	t.Run("rejects empty repo name", func(t *testing.T) {
		err := CreateRepo(context.Background(), "github_pat_xxxx", "")
		if err == nil {
			t.Fatal("expected error for empty repo name")
		}
	})
}

func TestFetchProfileName(t *testing.T) {
	t.Run("returns name on 200", func(t *testing.T) {
		_, cleanup := withFakeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/users/alice" {
				t.Errorf("path = %q, want /users/alice", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"login":"alice","name":"Alice Smith"}`))
		}))
		defer cleanup()

		name, err := FetchProfileName(context.Background(), "github_pat_xxxx", "alice")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "Alice Smith" {
			t.Errorf("name = %q, want Alice Smith", name)
		}
	})

	t.Run("returns empty on 404 without error", func(t *testing.T) {
		_, cleanup := withFakeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer cleanup()

		name, err := FetchProfileName(context.Background(), "github_pat_xxxx", "ghost")
		if err != nil {
			t.Fatalf("404 should not return an error, got %v", err)
		}
		if name != "" {
			t.Errorf("name = %q, want empty", name)
		}
	})

	t.Run("returns empty when name field is null", func(t *testing.T) {
		_, cleanup := withFakeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"login":"alice","name":null}`))
		}))
		defer cleanup()

		name, err := FetchProfileName(context.Background(), "github_pat_xxxx", "alice")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "" {
			t.Errorf("name = %q, want empty for null", name)
		}
	})

	t.Run("rejects forbidden chars in username", func(t *testing.T) {
		_, err := FetchProfileName(context.Background(), "github_pat_xxxx", "alice/../admin")
		if err == nil {
			t.Fatal("expected error for slash in username")
		}
	})
}

func TestDeleteRepo(t *testing.T) {
	t.Run("success on 204", func(t *testing.T) {
		var gotMethod, gotPath string
		_, cleanup := withFakeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}))
		defer cleanup()

		if err := DeleteRepo(context.Background(), "github_pat_xxxx", "alice", "myrepo"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotMethod != "DELETE" {
			t.Errorf("method = %q, want DELETE", gotMethod)
		}
		if gotPath != "/repos/alice/myrepo" {
			t.Errorf("path = %q, want /repos/alice/myrepo", gotPath)
		}
	})

	t.Run("error on 404", func(t *testing.T) {
		_, cleanup := withFakeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		}))
		defer cleanup()

		err := DeleteRepo(context.Background(), "github_pat_xxxx", "alice", "missing")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "404") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("rejects forbidden chars in owner", func(t *testing.T) {
		err := DeleteRepo(context.Background(), "github_pat_xxxx", "alice/../bob", "repo")
		if err == nil {
			t.Fatal("expected error for slash in owner")
		}
		if !strings.Contains(err.Error(), "forbidden characters") {
			t.Errorf("error %q does not mention forbidden characters", err.Error())
		}
	})

	t.Run("rejects forbidden chars in repo name", func(t *testing.T) {
		err := DeleteRepo(context.Background(), "github_pat_xxxx", "alice", "repo?force=true")
		if err == nil {
			t.Fatal("expected error for query char in repo name")
		}
	})
}
