package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "v2v-helper-cli",
	Short: "A CLI tool to generate Kubernetes Pod YAML",
	Long:  `A CLI tool to generate Kubernetes Pod YAML based on user input and admin file.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Register your commands here
	rootCmd.AddCommand(generateCmd)

	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
