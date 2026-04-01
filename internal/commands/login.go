package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zackey-heuristics/gitfive-go/internal/auth"
	"github.com/zackey-heuristics/gitfive-go/internal/httpclient"
)

// NewLoginCmd creates the "login" subcommand.
func NewLoginCmd() *cobra.Command {
	var clean bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate to GitHub",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := auth.NewCredentials()
			if err != nil {
				return err
			}

			if clean {
				_ = creds.Clean()
				fmt.Println("[+] Credentials and session files deleted!")
				return nil
			}

			client := httpclient.New()
			creds.Load()

			if len(creds.Session) > 0 {
				client.SetCookies("https://github.com", creds.Session)
			}

			valid, _ := auth.CheckSession(cmd.Context(), client)
			if valid {
				fmt.Println("[+] Creds are working!")
				fmt.Print("Do you want to re-login anyway? (Y/n): ")
				var choice string
				_, _ = fmt.Scanln(&choice)
				if choice == "" || choice == "y" || choice == "Y" {
					fmt.Println()
					creds2, _ := auth.NewCredentials()
					client2 := httpclient.New()
					return auth.Login(cmd.Context(), creds2, client2, true)
				}
				fmt.Println("\nBye!")
				return nil
			}

			fmt.Println("[-] Creds aren't active anymore. Relogin...")
			return auth.Login(cmd.Context(), creds, client, false)
		},
	}

	cmd.Flags().BoolVar(&clean, "clean", false, "Clear credentials and session files")
	return cmd
}
