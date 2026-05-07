package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zackey-heuristics/gitfive-go/internal/auth"
	"github.com/zackey-heuristics/gitfive-go/internal/credstore"
)

// NewLoginCmd creates the "login" subcommand.
func NewLoginCmd() *cobra.Command {
	var clean bool
	var force bool
	var useFileStorage bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate to GitHub",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := auth.NewCredentials()
			if err != nil {
				return err
			}

			// Apply backend selection BEFORE Save runs. The default is
			// keyring; --use-file-storage forces the on-disk fallback even
			// on hosts where keyring is available.
			if useFileStorage {
				creds.PreferredBackend = credstore.BackendFile
			} else {
				creds.PreferredBackend = credstore.BackendKeyring
			}

			if clean {
				_ = creds.Clean()
				fmt.Println("[+] Credentials cleared (keyring entry and file).")
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

	cmd.Flags().BoolVar(&clean, "clean", false, "Clear the credentials (both keyring entry and file)")
	cmd.Flags().BoolVar(&force, "force", false, "Re-authenticate even if a valid token is already saved")
	cmd.Flags().BoolVar(&useFileStorage, "use-file-storage", false,
		"Force base64 file storage instead of the OS keyring (useful for headless / CI hosts)")
	return cmd
}
