package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/term"

	"github.com/zackey-heuristics/gitfive-go/internal/httpclient"
	"github.com/zackey-heuristics/gitfive-go/internal/ui"
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

// PromptCreds interactively asks the user for username, password, and token.
func PromptCreds(creds *Credentials) {
	for creds.Username == "" {
		fmt.Print("Username => ")
		_, _ = fmt.Scanln(&creds.Username)
	}
	for creds.Password == "" {
		fmt.Print("Password => ")
		pw, _ := term.ReadPassword(int(inputFd()))
		fmt.Println()
		creds.Password = string(pw)
	}
	fmt.Println("Create a fine-grained personal access token (token starts with `github_pat_`):")
	fmt.Println("  - Resource owner: yourself")
	fmt.Println("  - Repository access: All repositories")
	fmt.Println("  - Repository permissions: Contents = Read and write, Metadata = Read")
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
	// Normalize to whole days using UTC dates so the result is stable
	// regardless of the local timezone.
	startOfDay := func(t time.Time) time.Time {
		t = t.UTC()
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
	days := int(startOfDay(expiry).Sub(startOfDay(now)).Hours() / 24)
	switch {
	case days < 0:
		return fmt.Sprintf("[!] Your fine-grained PAT expired on %s. Regenerate at %s", expiryDate, finePATSettingsURL)
	case days == 0:
		return fmt.Sprintf("[!] Your fine-grained PAT expires today (%s). Regenerate at %s", expiryDate, finePATSettingsURL)
	case days <= expiryWarnThresholdDays:
		return fmt.Sprintf("[!] Your fine-grained PAT expires in %d days (%s). Regenerate at %s", days, expiryDate, finePATSettingsURL)
	}
	return ""
}

// CheckToken validates the API token against GitHub. It enforces that the
// token is a fine-grained PAT (rejecting classic / server tokens), confirms
// the token belongs to the configured user, and warns when expiration is near.
func CheckToken(creds *Credentials) error {
	fmt.Println("Checking API token validity...")

	if err := validateTokenPrefix(creds.Token); err != nil {
		return err
	}

	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+creds.Token)
	resp, err := http.DefaultClient.Do(req)
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

	body, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("parse token response: %w", err)
	}

	owner, _ := data["login"].(string)
	// GitHub usernames are case-insensitive, so EqualFold is the correct
	// identity check here.
	if !strings.EqualFold(owner, creds.Username) {
		return fmt.Errorf("token owner (%s) doesn't match logged user (%s)", owner, creds.Username)
	}

	fmt.Println("[+] Token valid!")
	if w := expirationWarning(resp.Header.Get("github-authentication-token-expiration"), time.Now()); w != "" {
		fmt.Println(w)
	}
	fmt.Println()
	return nil
}

// Login performs the full GitHub web login flow including 2FA handling.
func Login(ctx context.Context, creds *Credentials, client *httpclient.Client, force bool) error {
	if !force {
		if creds.AreLoaded() {
			fmt.Println("[+] Credentials found!")
			fmt.Println()
		} else {
			fmt.Println("[-] No saved credentials found")
			fmt.Println()
			PromptCreds(creds)
		}
	} else {
		PromptCreds(creds)
	}

	if err := CheckToken(creds); err != nil {
		return err
	}

	// Fetch login page for authenticity token
	resp, err := client.Get(ctx, "https://github.com/login")
	if err != nil {
		return fmt.Errorf("login page fetch failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("login page parse failed: %w", err)
	}

	token, exists := doc.Find(`form[action="/session"] input[name="authenticity_token"]`).Attr("value")
	if !exists {
		return fmt.Errorf("authenticity token not found on login page")
	}

	// Submit login
	noRedirClient := httpclient.NewNoRedirect()
	// Copy cookies from the main client
	noRedirClient.HTTP.Jar = client.HTTP.Jar

	loginResp, err := noRedirClient.PostForm(ctx, "https://github.com/session", url.Values{
		"commit":             {"Sign+in"},
		"authenticity_token": {token},
		"login":              {creds.Username},
		"password":           {creds.Password},
	})
	if err != nil {
		return fmt.Errorf("login submit failed: %w", err)
	}
	defer func() { _ = loginResp.Body.Close() }()

	if loginResp.StatusCode != http.StatusFound {
		return fmt.Errorf("login failed, verify your credentials")
	}

	location := loginResp.Header.Get("Location")

	// Check for direct login success
	if getCookie(loginResp, "logged_in") == "yes" {
		return saveLoginSession(creds, loginResp, client)
	}

	tmprinter := ui.NewTMPrinter()

	switch {
	case location == "https://github.com/sessions/verified-device":
		return handleDeviceVerification(ctx, creds, client, noRedirClient)
	case location == "https://github.com/sessions/two-factor/app":
		return handleTOTP(ctx, creds, client)
	case strings.HasPrefix(location, "https://github.com/sessions/two-factor/mobile"):
		return handleMobile2FA(ctx, creds, client, tmprinter)
	case strings.HasPrefix(location, "https://github.com/sessions/two-factor"):
		return handleGeneric2FA(ctx, creds, client)
	default:
		return fmt.Errorf("unrecognized security step (location: %s)", location)
	}
}

func handleDeviceVerification(ctx context.Context, creds *Credentials, client, noRedirClient *httpclient.Client) error {
	fmt.Println("[*] Additional check (device verification)")
	creds.Session["_device_id"] = client.GetCookie("https://github.com", "_device_id")
	_ = creds.Save()

	resp, err := noRedirClient.Get(ctx, "https://github.com/sessions/verified-device")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	doc, _ := goquery.NewDocumentFromReader(resp.Body)
	token, _ := doc.Find(`form[action="/sessions/verified-device"] input[name="authenticity_token"]`).Attr("value")
	msg := doc.Find("#device-verification-prompt").Text()
	fmt.Printf("GitHub: \"%s\"\n", strings.TrimSpace(msg))

	fmt.Print("Code => ")
	otp, _ := term.ReadPassword(int(inputFd()))
	fmt.Println()

	postResp, err := client.PostForm(ctx, "https://github.com/sessions/verified-device", url.Values{
		"authenticity_token": {token},
		"otp":                {string(otp)},
	})
	if err != nil {
		return err
	}
	defer func() { _ = postResp.Body.Close() }()

	if getCookie(postResp, "logged_in") == "yes" {
		return saveLoginSession(creds, postResp, client)
	}
	return fmt.Errorf("wrong code, please retry")
}

func handleTOTP(ctx context.Context, creds *Credentials, client *httpclient.Client) error {
	fmt.Println("[*] Additional check (TOTP)")
	creds.Session["_device_id"] = client.GetCookie("https://github.com", "_device_id")
	_ = creds.Save()

	resp, err := client.Get(ctx, "https://github.com/sessions/two-factor/app")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	doc, _ := goquery.NewDocumentFromReader(resp.Body)
	token, _ := doc.Find(`form[action="/sessions/two-factor"] input[name="authenticity_token"]`).Attr("value")
	msg := doc.Find(`form[action="/sessions/two-factor"] div.mt-3`).Text()
	fmt.Printf("GitHub: \"%s\"\n", strings.TrimSpace(msg))

	fmt.Print("Code => ")
	otp, _ := term.ReadPassword(int(inputFd()))
	fmt.Println()

	postResp, err := client.PostForm(ctx, "https://github.com/sessions/two-factor", url.Values{
		"authenticity_token": {token},
		"app_otp":            {string(otp)},
	})
	if err != nil {
		return err
	}
	defer func() { _ = postResp.Body.Close() }()

	if getCookie(postResp, "logged_in") == "yes" {
		return saveLoginSession(creds, postResp, client)
	}
	return fmt.Errorf("wrong code, please retry")
}

func handleMobile2FA(ctx context.Context, creds *Credentials, client *httpclient.Client, tmprinter *ui.TMPrinter) error {
	fmt.Println("[*] 2FA detected (GitHub App)")
	creds.Session["_device_id"] = client.GetCookie("https://github.com", "_device_id")
	_ = creds.Save()

	resp, err := client.Get(ctx, "https://github.com/sessions/two-factor/mobile?auto=true")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusFound {
		return fmt.Errorf("temporarily rate limited, please wait a minute")
	}

	doc, _ := goquery.NewDocumentFromReader(resp.Body)
	token, _ := doc.Find(`form[action="/sessions/two-factor/mobile_poll"] input[name="authenticity_token"]`).Attr("value")
	msg := doc.Find(`p[data-target="sudo-credential-options.githubMobileChallengeMessage"]`).Text()
	number := doc.Find(`h3[data-target="sudo-credential-options.githubMobileChallengeValue"]`).Text()
	fmt.Printf("GitHub: \"%s\"\n", strings.TrimSpace(msg))
	fmt.Printf("Digits: %s\n\n", strings.TrimSpace(number))
	tmprinter.Out("Waiting for user confirmation...")

	for {
		time.Sleep(2 * time.Second)
		pollResp, err := client.PostForm(ctx, "https://github.com/sessions/two-factor/mobile_poll", url.Values{
			"authenticity_token": {token},
		})
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(pollResp.Body)
		_ = pollResp.Body.Close()

		var result map[string]string
		_ = json.Unmarshal(body, &result)

		switch result["status"] {
		case "STATUS_ACTIVE":
			continue
		case "STATUS_EXPIRED":
			tmprinter.Clear()
			return fmt.Errorf("2FA expired")
		case "STATUS_NOT_FOUND":
			tmprinter.Clear()
			return fmt.Errorf("request rejected")
		case "STATUS_APPROVED":
			tmprinter.Clear()
			fmt.Println("[+] Got confirmation!")
			return saveLoginSession(creds, pollResp, client)
		}
	}
}

func handleGeneric2FA(ctx context.Context, creds *Credentials, client *httpclient.Client) error {
	fmt.Println("[*] 2FA")
	creds.Session["_device_id"] = client.GetCookie("https://github.com", "_device_id")
	_ = creds.Save()

	resp, err := client.Get(ctx, "https://github.com/sessions/two-factor/app")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	doc, _ := goquery.NewDocumentFromReader(resp.Body)
	token, _ := doc.Find(`form[action="/sessions/two-factor"] input[name="authenticity_token"]`).Attr("value")
	msg := strings.TrimSpace(strings.Split(
		doc.Find(`form[action="/sessions/two-factor"] div.mt-3`).Text(), "\n",
	)[0])
	fmt.Printf("GitHub: \"%s\"\n", msg)

	fmt.Print("Code => ")
	otp, _ := term.ReadPassword(int(inputFd()))
	fmt.Println()

	postResp, err := client.PostForm(ctx, "https://github.com/sessions/two-factor", url.Values{
		"authenticity_token": {token},
		"otp":                {string(otp)},
	})
	if err != nil {
		return err
	}
	defer func() { _ = postResp.Body.Close() }()

	if getCookie(postResp, "logged_in") == "yes" {
		return saveLoginSession(creds, postResp, client)
	}
	return fmt.Errorf("wrong code, please retry")
}

func saveLoginSession(creds *Credentials, resp *http.Response, client *httpclient.Client) error {
	creds.Session["user_session"] = getCookie(resp, "user_session")
	creds.Session["__Host-user_session_same_site"] = getCookie(resp, "__Host-user_session_same_site")
	creds.Session["_device_id"] = client.GetCookie("https://github.com", "_device_id")
	if err := creds.Save(); err != nil {
		return err
	}
	fmt.Println("[+] Logged in!")
	fmt.Printf("[+] Credentials saved in %s\n", creds.CredsPath())
	fmt.Printf("[+] Session saved in %s\n", creds.SessionPath())
	return nil
}

func getCookie(resp *http.Response, name string) string {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

func inputFd() uintptr {
	return os.Stdin.Fd()
}
