package cli

import (
	"context"
	"fmt"

	"github.com/platform9/vjailbreak/pkg/vpwned/openapiv3/proto/service/api"
	"github.com/platform9/vjailbreak/pkg/vpwned/server"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "show version",
	Long:  "show version",
	Run: func(cmd *cobra.Command, args []string) {
		v := server.VpwnedVersion{}
		ver, _ := v.Version(context.Background(), &api.VersionRequest{})
		fmt.Println("vpwctl version", ver.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
