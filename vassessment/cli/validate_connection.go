package cli

import (
	"fmt"

	"github.com/platform9/vjailbreak/vassessment/preflight"
	"github.com/spf13/cobra"
)

var validateConnectionCmd = &cobra.Command{
	Use:   "connection",
	Short: "Validate vCenter host/username/password input shape (no network call)",
	RunE: func(cmd *cobra.Command, args []string) error {
		host, _ := cmd.Flags().GetString("host")
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")

		in := preflight.ConnectionInput{Host: host, Username: username, Password: password}
		if err := preflight.ValidateConnectionInput(in); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "connection input OK")
		return nil
	},
}

func init() {
	validateConnectionCmd.Flags().String("host", "", "vCenter host, FQDN, IP, or URL")
	validateConnectionCmd.Flags().String("username", "", "vCenter username")
	validateConnectionCmd.Flags().String("password", "", "vCenter password")
	validateCmd.AddCommand(validateConnectionCmd)
}
