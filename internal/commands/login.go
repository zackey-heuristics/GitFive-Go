package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zackey-heuristics/gitfive-go/internal/auth"
)

// NewLoginCmd creates the "login" subcommand.
func NewLoginCmd() *cobra.Command {
	var clean bool
	var force bool

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
				fmt.Println("[+] Credentials file deleted!")
				return nil
			}

			creds.Load()

			if !force && creds.AreLoaded() {
				fmt.Println("[+] Existing token found, validating...")
				if err := auth.CheckToken(creds); err == nil {
					fmt.Print("Do you want to re-authenticate anyway? (y/N): ")
					var choice string
					_, _ = fmt.Scanln(&choice)
					if choice != "y" && choice != "Y" {
						fmt.Println("Bye!")
						return nil
					}
				} else {
					fmt.Printf("[-] Saved token is no longer valid: %v\n", err)
				}
			}

			return auth.Login(creds, true)
		},
	}

	cmd.Flags().BoolVar(&clean, "clean", false, "Clear the credentials file")
	cmd.Flags().BoolVar(&force, "force", false, "Re-authenticate even if a valid token is already saved")
	return cmd
}
