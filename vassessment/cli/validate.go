package cli

import "github.com/spf13/cobra"

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Run preflight checks that don't require a live vCenter connection",
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
