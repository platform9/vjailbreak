package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "v2v-helper-cli",
	Short: "A CLI tool to migrate VMs from VMware to Openstack.",
	Long:  `A CLI tool to migrate VMs from VMware to Openstack.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Register your commands here
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(statusCmd)

	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
