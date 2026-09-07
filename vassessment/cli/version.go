package cli

import (
	"fmt"

	"github.com/platform9/vjailbreak/vassessment/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the vAssessment version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), "vassessment version", version.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
