// Package cli wires up the vAssessment command-line interface.
package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "vassessment",
	Short: "vAssessment — discovery & migration assessment for vJailbreak",
	Long: `vAssessment inventories a VMware estate (read-only) and evaluates it
against vJailbreak's migration pre-check catalog, producing a per-VM
readiness verdict and a shareable report.`,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
