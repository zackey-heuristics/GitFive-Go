package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/zackey-heuristics/gitfive-go/internal/credstore"
)

const (
	fineGrainedTokenPrefix  = "github_pat_"
	finePATDocsURL          = "https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens#creating-a-fine-grained-personal-access-token"
	finePATSettingsURL      = "https://github.com/settings/tokens?type=beta"
	expiryWarnThresholdDays = 30
)

// classicTokenPrefixes lists token-prefix patterns that this tool no longer
// accepts (classic PATs and server-side OAuth tokens).
var classicTokenPrefixes = []string{"ghp_", "gho_", "ghs_", "ghu_", "ghr_"}

// authHTTPClient enforces an explicit timeout because http.DefaultClient has
// none; combined with request-context cancellation it bounds worst-case wait
// even for misbehaving connections.
var authHTTPClient = &http.Client{Timeout: 60 * time.Second}

// PromptCreds interactively asks for the fine-grained PAT. The owner's
// username is derived from the token by CheckToken, so it is not asked here.
func PromptCreds(creds *Credentials) {
	fmt.Println("Create a fine-grained personal access token (token starts with `github_pat_`):")
	fmt.Println("  - Resource owner: yourself")
	fmt.Println("  - Repository access: All repositories")
	fmt.Println("  - Repository permissions:")
	fmt.Println("      Contents       = Read and write")
	fmt.Println("      Administration = Read and write")
	fmt.Println("      Metadata       = Read")
	fmt.Println("See:", finePATDocsURL)
	for creds.Token == "" {
		fmt.Print("API Token => ")
		tok, _ := term.ReadPassword(int(inputFd()))
		fmt.Println()
		creds.Token = strings.TrimSpace(string(tok))
	}
	fmt.Println()
}

// minFineGrainedTokenLen is a sanity floor; real fine-grained PATs are
// around 90+ characters. Anything shorter is almost certainly malformed.
const minFineGrainedTokenLen = 40

// validateTokenPrefix accepts only fine-grained PATs and rejects classic /
// server-issued tokens with an actionable error message. The caller is
// expected to have already trimmed the token, but we trim defensively to
// avoid surprises from pasted whitespace.
func validateTokenPrefix(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token is empty")
	}
	for _, p := range classicTokenPrefixes {
		if strings.HasPrefix(token, p) {
			return fmt.Errorf(
				"classic or server token detected (prefix %q); GitFive-Go now requires a fine-grained PAT (prefix \"github_pat_\"). Create one at %s",
				p, finePATSettingsURL,
			)
		}
	}
	if !strings.HasPrefix(token, fineGrainedTokenPrefix) {
		return fmt.Errorf(
			"unrecognized token format; GitFive-Go requires a fine-grained PAT (prefix \"github_pat_\"). Create one at %s",
			finePATSettingsURL,
		)
	}
	if len(token) < minFineGrainedTokenLen {
		return fmt.Errorf("token is implausibly short (%d chars); fine-grained PATs are much longer — re-paste it", len(token))
	}
	return nil
}

// expirationWarning returns a one-line warning if the fine-grained PAT
// expiration is within expiryWarnThresholdDays (or already past). Empty
// string means "no warning" (header missing, unparseable, or far enough away).
func expirationWarning(headerVal string, now time.Time) string {
	if headerVal == "" {
		return ""
	}
	// GitHub's documented format is "2026-05-18 12:00:00 UTC". Accept that
	// (literal UTC suffix) plus a numeric-offset variant and RFC3339.
	// Avoid the Go `MST` layout token: time.Parse silently assigns offset 0
	// to non-UTC three-letter abbreviations it doesn't know, which would
	// corrupt expiry math if the upstream format ever changes.
	headerVal = strings.TrimSpace(headerVal)
	layouts := []string{
		"2006-01-02 15:04:05 UTC",
		"2006-01-02 15:04:05 -0700",
		time.RFC3339,
	}
	var (
		expiry time.Time
		parsed bool
	)
	for _, l := range layouts {
		t, err := time.Parse(l, headerVal)
		if err == nil {
			// time.Parse defaults to UTC for layouts without a tz indicator,
			// which matches the literal "UTC" layout's intent. Numeric and
			// RFC3339 offsets are preserved.
			expiry = t
			parsed = true
			break
		}
	}
	if !parsed {
		return ""
	}
	expiryDate := expiry.Format("2006-01-02")
	// Decide expired vs upcoming on the actual instant first: a token whose
	// expiry timestamp is already in the past must be reported as "expired"
	// even when it falls on the same calendar day as `now`, so users are not
	// misled into thinking an already-401 token is still valid.
	if expiry.Before(now) {
		return fmt.Sprintf("[!] Your fine-grained PAT expired on %s. Regenerate at %s", expiryDate, finePATSettingsURL)
	}
	// For warnings about an upcoming expiry, count whole UTC calendar days
	// so the message is stable regardless of the local timezone.
	startOfDay := func(t time.Time) time.Time {
		t = t.UTC()
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
	days := int(startOfDay(expiry).Sub(startOfDay(now)).Hours() / 24)
	switch {
	case days == 0:
		return fmt.Sprintf("[!] Your fine-grained PAT expires today (%s). Regenerate at %s", expiryDate, finePATSettingsURL)
	case days <= expiryWarnThresholdDays:
		return fmt.Sprintf("[!] Your fine-grained PAT expires in %d days (%s). Regenerate at %s", days, expiryDate, finePATSettingsURL)
	}
	return ""
}

// CheckToken validates the API token against GitHub. It enforces that the
// token is a fine-grained PAT (rejecting classic / server tokens), populates
// creds.Username from the `GET /user` response, and warns when the token
// expiration is near.
func CheckToken(creds *Credentials) error {
	fmt.Println("Checking API token validity...")

	// Trim once on the canonical field so the same value is used by both
	// validateTokenPrefix and the Authorization header below. Loaded creds
	// (read from disk) may carry whitespace if the file was hand-edited;
	// PromptCreds-supplied tokens are already trimmed but trimming again is
	// idempotent.
	creds.Token = strings.TrimSpace(creds.Token)

	if err := validateTokenPrefix(creds.Token); err != nil {
		return err
	}

	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+creds.Token)
	resp, err := authHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("token check failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("token seems invalid (unauthorized); regenerate a fine-grained PAT at %s", finePATSettingsURL)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned unexpected status code (%d)", resp.StatusCode)
	}

	// Cap the response read to keep memory bounded. The /user JSON is small
	// (a few KB at most) so 64 KiB is generous.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("parse token response: %w", err)
	}

	login, _ := data["login"].(string)
	if login == "" {
		return fmt.Errorf("token response missing `login` field")
	}
	creds.Username = login

	fmt.Printf("[+] Token valid! (user: %s)\n", login)
	if w := expirationWarning(resp.Header.Get("github-authentication-token-expiration"), time.Now()); w != "" {
		fmt.Println(w)
	}
	fmt.Println()
	return nil
}

// Login validates the stored token (prompting for one if needed) and
// persists the resulting credentials. There is no longer any web/2FA flow:
// authentication is a single fine-grained PAT.
func Login(creds *Credentials, force bool) error {
	if force || !creds.AreLoaded() {
		// Reset token AND resolved username before reprompting so a stale
		// username from the prior token cannot persist if validation fails
		// midway. CheckToken repopulates Username on success.
		if force {
			creds.Token = ""
			creds.Username = ""
		}
		PromptCreds(creds)
	} else {
		fmt.Println("[+] Saved token found, validating...")
	}

	if err := CheckToken(creds); err != nil {
		return err
	}

	if err := creds.Save(); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	switch creds.ActiveBackend {
	case credstore.BackendKeyring:
		fmt.Printf("[+] Token stored in the OS keyring (marker file: %s)\n", creds.CredsPath())
	case credstore.BackendFile:
		fmt.Printf("[+] Token stored in %s (file storage)\n", creds.CredsPath())
	default:
		fmt.Printf("[+] Credentials saved in %s\n", creds.CredsPath())
	}
	return nil
}

func inputFd() uintptr {
	return os.Stdin.Fd()
}
