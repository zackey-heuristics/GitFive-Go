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
	"github.com/zackey-heuristics/gitfive-go/internal/util"
)

// PromptCreds interactively asks the user for username, password, and token.
func PromptCreds(creds *Credentials) {
	for creds.Username == "" {
		fmt.Print("Username => ")
		fmt.Scanln(&creds.Username)
	}
	for creds.Password == "" {
		fmt.Print("Password => ")
		pw, _ := term.ReadPassword(int(inputFd()))
		fmt.Println()
		creds.Password = string(pw)
	}
	fmt.Println(`The API token requires the "repo" and "delete_repo" scopes.`)
	fmt.Println("See: https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token")
	for creds.Token == "" {
		fmt.Print("API Token => ")
		tok, _ := term.ReadPassword(int(inputFd()))
		fmt.Println()
		creds.Token = string(tok)
	}
	fmt.Println()
}

// CheckToken validates the API token against GitHub and checks required scopes.
func CheckToken(creds *Credentials) error {
	fmt.Println("Checking API token validity...")
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+creds.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("token check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("token seems invalid")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub API returned unexpected status code (%d)", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	json.Unmarshal(body, &data)

	owner, _ := data["login"].(string)
	if !strings.EqualFold(owner, creds.Username) {
		return fmt.Errorf("token owner (%s) doesn't match logged user (%s)", owner, creds.Username)
	}

	rawScopes := resp.Header.Get("X-OAuth-Scopes")
	if rawScopes == "" {
		return fmt.Errorf("token seems invalid (no scopes)")
	}

	scopes := strings.Split(rawScopes, ",")
	for i := range scopes {
		scopes[i] = strings.TrimSpace(scopes[i])
	}

	required := map[string]bool{"repo": false, "delete_repo": false}
	var excessive []string
	for _, s := range scopes {
		if _, ok := required[s]; ok {
			required[s] = true
		} else {
			excessive = append(excessive, s)
		}
	}

	for scope, found := range required {
		if !found {
			return fmt.Errorf("token missing required scope: %s (current: %s)", scope, util.HumanizeList(scopes))
		}
	}

	var reqScopes []string
	for s := range required {
		reqScopes = append(reqScopes, s)
	}
	fmt.Printf("[+] Token valid! (scopes: %s)\n", util.HumanizeList(reqScopes))

	if len(excessive) > 0 {
		fmt.Printf("[!] Your token has excessive scopes (%s).\n", util.HumanizeList(excessive))
		fmt.Println("These scopes are not needed by GitFive, so keep it secure or generate a new one.")
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
	defer resp.Body.Close()

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
	defer loginResp.Body.Close()

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
	creds.Save()

	resp, err := noRedirClient.Get(ctx, "https://github.com/sessions/verified-device")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	doc, _ := goquery.NewDocumentFromReader(resp.Body)
	token, _ := doc.Find(`form[action="/sessions/verified-device"] input[name="authenticity_token"]`).Attr("value")
	msg := doc.Find("#device-verification-prompt").Text()
	fmt.Printf("GitHub: \"%s\"\n", strings.TrimSpace(msg))

	fmt.Print("Code => ")
	otp, _ := term.ReadPassword(int(inputFd()))
	fmt.Println()

	postResp, err := client.PostForm(ctx, "https://github.com/sessions/verified-device", url.Values{
		"authenticity_token": {token},
		"otp":               {string(otp)},
	})
	if err != nil {
		return err
	}
	defer postResp.Body.Close()

	if getCookie(postResp, "logged_in") == "yes" {
		return saveLoginSession(creds, postResp, client)
	}
	return fmt.Errorf("wrong code, please retry")
}

func handleTOTP(ctx context.Context, creds *Credentials, client *httpclient.Client) error {
	fmt.Println("[*] Additional check (TOTP)")
	creds.Session["_device_id"] = client.GetCookie("https://github.com", "_device_id")
	creds.Save()

	resp, err := client.Get(ctx, "https://github.com/sessions/two-factor/app")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	doc, _ := goquery.NewDocumentFromReader(resp.Body)
	token, _ := doc.Find(`form[action="/sessions/two-factor"] input[name="authenticity_token"]`).Attr("value")
	msg := doc.Find(`form[action="/sessions/two-factor"] div.mt-3`).Text()
	fmt.Printf("GitHub: \"%s\"\n", strings.TrimSpace(msg))

	fmt.Print("Code => ")
	otp, _ := term.ReadPassword(int(inputFd()))
	fmt.Println()

	postResp, err := client.PostForm(ctx, "https://github.com/sessions/two-factor", url.Values{
		"authenticity_token": {token},
		"app_otp":           {string(otp)},
	})
	if err != nil {
		return err
	}
	defer postResp.Body.Close()

	if getCookie(postResp, "logged_in") == "yes" {
		return saveLoginSession(creds, postResp, client)
	}
	return fmt.Errorf("wrong code, please retry")
}

func handleMobile2FA(ctx context.Context, creds *Credentials, client *httpclient.Client, tmprinter *ui.TMPrinter) error {
	fmt.Println("[*] 2FA detected (GitHub App)")
	creds.Session["_device_id"] = client.GetCookie("https://github.com", "_device_id")
	creds.Save()

	resp, err := client.Get(ctx, "https://github.com/sessions/two-factor/mobile?auto=true")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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
		pollResp.Body.Close()

		var result map[string]string
		json.Unmarshal(body, &result)

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
	creds.Save()

	resp, err := client.Get(ctx, "https://github.com/sessions/two-factor/app")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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
		"otp":               {string(otp)},
	})
	if err != nil {
		return err
	}
	defer postResp.Body.Close()

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
