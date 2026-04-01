package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/zackey-heuristics/gitfive-go/internal/httpclient"
)

// CheckSession validates whether the current session cookies are still active.
func CheckSession(ctx context.Context, client *httpclient.Client) (bool, error) {
	noRedir := httpclient.NewNoRedirect()
	noRedir.HTTP.Jar = client.HTTP.Jar

	resp, err := noRedir.Get(ctx, "https://github.com/settings/profile")
	if err != nil {
		return false, err
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

// CheckAndLogin validates the session and re-logs in if needed.
func CheckAndLogin(ctx context.Context, creds *Credentials, client *httpclient.Client) error {
	valid, err := CheckSession(ctx, client)
	if err != nil {
		return fmt.Errorf("session check failed: %w", err)
	}

	if !valid {
		fmt.Println("[-] Session expired, re-logging in...")
		if err := Login(ctx, creds, client, false); err != nil {
			return err
		}
		fmt.Println("[+] Session re-established!")
	} else {
		fmt.Println("[+] Session active.")
	}
	return nil
}
