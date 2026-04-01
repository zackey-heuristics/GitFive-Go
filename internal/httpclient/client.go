package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/zackey-heuristics/gitfive-go/internal/config"
)

// Client wraps net/http.Client with default headers and cookie persistence.
type Client struct {
	HTTP *http.Client
}

// New creates a Client with a cookie jar, default headers, and timeout.
func New() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		HTTP: &http.Client{
			Jar:     jar,
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
	}
}

// NewNoRedirect creates a Client that does not follow redirects.
func NewNoRedirect() *Client {
	c := New()
	c.HTTP.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return c
}

// Get performs a GET request with default headers.
func (c *Client) Get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	applyDefaultHeaders(req)
	return c.HTTP.Do(req)
}

// Post performs a POST request with default headers and the given body.
func (c *Client) Post(ctx context.Context, rawURL, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	applyDefaultHeaders(req)
	return c.HTTP.Do(req)
}

// PostForm performs a POST request with URL-encoded form data.
func (c *Client) PostForm(ctx context.Context, rawURL string, data url.Values) (*http.Response, error) {
	return c.Post(ctx, rawURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

// Head performs a HEAD request with default headers.
func (c *Client) Head(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return nil, err
	}
	applyDefaultHeaders(req)
	return c.HTTP.Do(req)
}

// Do performs a custom request.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	applyDefaultHeaders(req)
	return c.HTTP.Do(req)
}

// SetCookies sets cookies on the client's cookie jar for the given URL.
func (c *Client) SetCookies(rawURL string, cookies map[string]string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	var cs []*http.Cookie
	for k, v := range cookies {
		cs = append(cs, &http.Cookie{Name: k, Value: v})
	}
	c.HTTP.Jar.SetCookies(u, cs)
}

// GetCookie retrieves a named cookie from the jar for the given URL.
func (c *Client) GetCookie(rawURL, name string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	for _, cookie := range c.HTTP.Jar.Cookies(u) {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func applyDefaultHeaders(req *http.Request) {
	for key, vals := range config.DefaultHeaders {
		if req.Header.Get(key) == "" {
			for _, v := range vals {
				req.Header.Set(key, v)
			}
		}
	}
}
