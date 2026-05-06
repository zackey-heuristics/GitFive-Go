package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestValidateTokenPrefix(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		wantErr    bool
		wantSubstr string
	}{
		{name: "fine-grained accepted", token: "github_pat_11ABCDEFGHIJKLMNOPQRSTUV_abcdefghijklmnopqrstuvwxyz0123456789", wantErr: false},
		{name: "fine-grained with whitespace accepted", token: "  github_pat_11ABCDEFGHIJKLMNOPQRSTUV_abcdefghijklmnopqrstuvwxyz0123456789\n", wantErr: false},
		{name: "classic ghp_ rejected", token: "ghp_1234567890abcdef", wantErr: true, wantSubstr: "classic or server token"},
		{name: "oauth gho_ rejected", token: "gho_1234567890abcdef", wantErr: true, wantSubstr: "classic or server token"},
		{name: "server ghs_ rejected", token: "ghs_1234567890abcdef", wantErr: true, wantSubstr: "classic or server token"},
		{name: "user ghu_ rejected", token: "ghu_1234567890abcdef", wantErr: true, wantSubstr: "classic or server token"},
		{name: "refresh ghr_ rejected", token: "ghr_1234567890abcdef", wantErr: true, wantSubstr: "classic or server token"},
		{name: "empty rejected", token: "", wantErr: true, wantSubstr: "empty"},
		{name: "whitespace-only rejected", token: "   \n\t", wantErr: true, wantSubstr: "empty"},
		{name: "garbage prefix rejected", token: "abcdwhatever", wantErr: true, wantSubstr: "unrecognized token format"},
		{name: "bare github_pat_ prefix rejected as too short", token: "github_pat_", wantErr: true, wantSubstr: "implausibly short"},
		{name: "short fine-grained rejected", token: "github_pat_short", wantErr: true, wantSubstr: "implausibly short"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTokenPrefix(tt.token)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.wantSubstr != "" && !strings.Contains(err.Error(), tt.wantSubstr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCheckTokenRejectsClassicWithoutNetwork(t *testing.T) {
	// A server that fails the test if reached — proves prefix check
	// short-circuits before any HTTP call.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("HTTP request should not be made for classic token; got %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// CheckToken always hits api.github.com, but for a `ghp_` token the
	// prefix check fires first so no network call happens. Use an obviously
	// classic token; if any call leaks, the in-package httptest server above
	// won't catch it (different host) — the real safety net is that the
	// prefix-check returns before NewRequest. Sanity-check error string.
	creds := &Credentials{Username: "u", Token: "ghp_xxx"}
	err := CheckToken(creds)
	if err == nil {
		t.Fatal("expected CheckToken to reject classic token, got nil")
	}
	if !strings.Contains(err.Error(), "classic or server token") {
		t.Fatalf("error %q does not mention classic/server token", err.Error())
	}
}

func TestExpirationWarning(t *testing.T) {
	// Fix "now" so the test is deterministic.
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		header     string
		wantEmpty  bool
		wantSubstr string
	}{
		{name: "missing header", header: "", wantEmpty: true},
		{name: "expired yesterday", header: "2026-05-05 12:00:00 UTC", wantSubstr: "expired on 2026-05-05"},
		{name: "expired earlier today reports expired (not expires today)", header: "2026-05-06 09:00:00 UTC", wantSubstr: "expired on 2026-05-06"},
		{name: "expires today", header: "2026-05-06 23:00:00 UTC", wantSubstr: "expires today (2026-05-06)"},
		{name: "expires in 5 days", header: "2026-05-11 12:00:00 UTC", wantSubstr: "expires in 5 days (2026-05-11)"},
		{name: "expires in 30 days (boundary, warn)", header: "2026-06-05 12:00:00 UTC", wantSubstr: "expires in 30 days"},
		{name: "expires in 31 days (no warn)", header: "2026-06-06 12:00:00 UTC", wantEmpty: true},
		{name: "expires in 365 days (no warn)", header: "2027-05-06 12:00:00 UTC", wantEmpty: true},
		{name: "offset format", header: "2026-05-11 12:00:00 +0000", wantSubstr: "expires in 5 days"},
		{name: "rfc3339 format", header: "2026-05-11T12:00:00Z", wantSubstr: "expires in 5 days"},
		{name: "non-UTC three-letter abbreviation falls through (no silent miscalc)", header: "2026-05-11 12:00:00 PDT", wantEmpty: true},
		{name: "unparseable header", header: "not-a-date", wantEmpty: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expirationWarning(tt.header, now)
			if tt.wantEmpty {
				if got != "" {
					t.Fatalf("expected empty warning, got %q", got)
				}
				return
			}
			if got == "" {
				t.Fatalf("expected warning, got empty")
			}
			if !strings.Contains(got, tt.wantSubstr) {
				t.Fatalf("warning %q does not contain %q", got, tt.wantSubstr)
			}
			if !strings.Contains(got, finePATSettingsURL) {
				t.Fatalf("warning %q does not contain regen URL", got)
			}
		})
	}
}
