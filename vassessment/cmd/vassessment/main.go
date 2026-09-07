// Command vassessment is the vAssessment CLI entry point.
package main

import (
	"fmt"
	"os"

	"github.com/platform9/vjailbreak/vassessment/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
