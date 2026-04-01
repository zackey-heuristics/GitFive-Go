package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zackey-heuristics/gitfive-go/internal/auth"
	"github.com/zackey-heuristics/gitfive-go/internal/ui"
)

// clientPool holds a set of HTTP clients that can be rotated on rate limit.
type clientPool struct {
	magazine []*http.Client
	loaded   *http.Client
}

// Interface manages multi-client GitHub API access with auto rate-limit switching.
type Interface struct {
	clients   map[string]*clientPool
	tmprinter *ui.TMPrinter
}

// NewInterface creates an API interface with authenticated and unauthenticated clients.
func NewInterface(creds *auth.Credentials, tmprinter *ui.TMPrinter) *Interface {
	authClient := &http.Client{Timeout: 30 * time.Second}
	unauthClient := &http.Client{Timeout: 30 * time.Second}

	// Store the token for the authenticated client via a custom transport
	authClient.Transport = &tokenTransport{token: creds.Token, base: http.DefaultTransport}

	authPool := &clientPool{
		magazine: []*http.Client{authClient},
		loaded:   authClient,
	}
	unauthPool := &clientPool{
		magazine: []*http.Client{unauthClient},
		loaded:   unauthClient,
	}
	allPool := &clientPool{
		magazine: append(authPool.magazine, unauthPool.magazine...),
		loaded:   authClient,
	}

	return &Interface{
		clients: map[string]*clientPool{
			"authenticated":   authPool,
			"unauthenticated": unauthPool,
			"all":             allPool,
		},
		tmprinter: tmprinter,
	}
}

// Query performs a GET request to the GitHub API and returns parsed JSON.
// It auto-retries on rate limit (403) by switching clients.
func (api *Interface) Query(ctx context.Context, path string, connType string) (json.RawMessage, error) {
	if connType == "" {
		connType = "all"
	}

	pool := api.clients[connType]
	if pool == nil {
		return nil, fmt.Errorf("unknown connection type: %s", connType)
	}

	for {
		req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com"+path, nil)
		if err != nil {
			return nil, err
		}

		resp, err := pool.loaded.Do(req)
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == 403 {
			resource := resp.Header.Get("X-RateLimit-Resource")
			if err := api.waitAndReloadClient(ctx, connType, resource); err != nil {
				return nil, err
			}
			continue
		}

		if resp.StatusCode == 200 || resp.StatusCode == 404 || resp.StatusCode == 422 {
			return json.RawMessage(body), nil
		}

		return nil, fmt.Errorf("API returned unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

func (api *Interface) waitAndReloadClient(ctx context.Context, connType, resource string) error {
	pool := api.clients[connType]
	for {
		for _, client := range pool.magazine {
			remaining, err := api.verifyRateLimit(ctx, resource, client)
			if err != nil {
				continue
			}
			if remaining {
				pool.loaded = client
				api.tmprinter.Clear()
				return nil
			}
		}
		api.tmprinter.Out("Waiting for an API client to be reloaded...")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (api *Interface) verifyRateLimit(ctx context.Context, resource string, client *http.Client) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/rate_limit", nil)
	if err != nil {
		return false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var data struct {
		Resources map[string]struct {
			Remaining int `json:"remaining"`
		} `json:"resources"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return false, err
	}

	r, ok := data.Resources[resource]
	if !ok {
		return true, nil // Unknown resource, assume OK
	}
	return r.Remaining > 0, nil
}

// tokenTransport adds Bearer auth to requests.
type tokenTransport struct {
	token string
	base  http.RoundTripper
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req2)
}
