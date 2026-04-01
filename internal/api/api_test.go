package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/zackey-heuristics/gitfive-go/internal/auth"
	"github.com/zackey-heuristics/gitfive-go/internal/ui"
)

func TestQuerySuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/testuser" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"login": "testuser"})
	}))
	defer srv.Close()

	api := newTestAPI(srv.URL, "faketoken")

	data, err := api.Query(context.Background(), "/users/testuser", "all")
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]string
	json.Unmarshal(data, &result)
	if result["login"] != "testuser" {
		t.Errorf("expected login 'testuser', got %v", result["login"])
	}
}

func TestQueryRateLimitRetry(t *testing.T) {
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := callCount.Add(1)

		if r.URL.Path == "/rate_limit" {
			// Return remaining > 0 so the client switches
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"resources": map[string]interface{}{
					"core": map[string]interface{}{"remaining": 100},
				},
			})
			return
		}

		if count == 1 {
			// First call: rate limited
			w.Header().Set("X-RateLimit-Resource", "core")
			w.WriteHeader(403)
			w.Write([]byte(`{"message":"rate limited"}`))
			return
		}

		// Second call: success
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	api := newTestAPI(srv.URL, "faketoken")

	data, err := api.Query(context.Background(), "/test", "all")
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]string
	json.Unmarshal(data, &result)
	if result["ok"] != "true" {
		t.Errorf("expected ok, got %v", result)
	}
}

func newTestAPI(baseURL, token string) *Interface {
	creds := &auth.Credentials{Token: token}
	api := NewInterface(creds, ui.NewTMPrinter())

	// Override the base URL for all clients
	for _, pool := range api.clients {
		for _, c := range pool.magazine {
			c.Transport = &rewriteTransport{
				base:    c.Transport,
				baseURL: baseURL,
			}
		}
		if pool.loaded != nil {
			pool.loaded.Transport = &rewriteTransport{
				base:    pool.loaded.Transport,
				baseURL: baseURL,
			}
		}
	}
	return api
}

// rewriteTransport rewrites API requests to point at a test server.
type rewriteTransport struct {
	base    http.RoundTripper
	baseURL string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = t.baseURL[len("http://"):]
	if t.base == nil {
		return http.DefaultTransport.RoundTrip(req2)
	}
	// Unwrap tokenTransport if nested
	if tt, ok := t.base.(*tokenTransport); ok {
		req2.Header.Set("Authorization", "Bearer "+tt.token)
		return http.DefaultTransport.RoundTrip(req2)
	}
	return http.DefaultTransport.RoundTrip(req2)
}
