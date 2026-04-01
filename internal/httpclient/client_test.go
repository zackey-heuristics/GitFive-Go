package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		ua := r.Header.Get("User-Agent")
		if ua == "" {
			t.Error("expected User-Agent header")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := New()
	resp, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("got %q, want %q", body, "ok")
	}
}

func TestPostForm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		r.ParseForm()
		if r.FormValue("key") != "value" {
			t.Errorf("expected key=value, got %s", r.FormValue("key"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New()
	resp, err := c.PostForm(context.Background(), srv.URL, map[string][]string{"key": {"value"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("got status %d, want 200", resp.StatusCode)
	}
}

func TestNoRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/other", http.StatusFound)
	}))
	defer srv.Close()

	c := NewNoRedirect()
	resp, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusFound)
	}
}

func TestSetAndGetCookie(t *testing.T) {
	c := New()
	c.SetCookies("https://github.com", map[string]string{"session": "abc123"})
	got := c.GetCookie("https://github.com", "session")
	if got != "abc123" {
		t.Errorf("got cookie %q, want %q", got, "abc123")
	}
	missing := c.GetCookie("https://github.com", "nonexistent")
	if missing != "" {
		t.Errorf("got cookie %q for missing key, want empty", missing)
	}
}
