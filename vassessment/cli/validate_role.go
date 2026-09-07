package cli

import (
	"fmt"

	"github.com/platform9/vjailbreak/vassessment/preflight"
	"github.com/spf13/cobra"
)

var validateRoleCmd = &cobra.Command{
	Use:   "role",
	Short: "Validate an exported vCenter role definition against a required-privileges file",
	Long: `Compares two role-definition JSON files ({"name": "...", "privileges": [...]}):
one exported from the role actually granted in vCenter, one listing what
vAssessment requires. Both are local files — this never contacts vCenter.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		grantedFile, _ := cmd.Flags().GetString("granted")
		requiredFile, _ := cmd.Flags().GetString("required")

		granted, err := preflight.LoadRoleDefinition(grantedFile)
		if err != nil {
			return err
		}
		required, err := preflight.LoadRoleDefinition(requiredFile)
		if err != nil {
			return err
		}

		missing, extra, ok := preflight.ValidatePrivileges(required.Privileges, granted.Privileges)
		out := cmd.OutOrStdout()
		if len(extra) > 0 {
			fmt.Fprintf(out, "role %q grants %d privilege(s) beyond what's required: %v\n", granted.Name, len(extra), extra)
		}
		if !ok {
			return fmt.Errorf("role %q is missing %d required privilege(s): %v", granted.Name, len(missing), missing)
		}
		fmt.Fprintf(out, "role %q satisfies all %d required privilege(s)\n", granted.Name, len(required.Privileges))
		return nil
	},
}

func init() {
	validateRoleCmd.Flags().String("granted", "", "path to the role definition actually granted in vCenter (required)")
	validateRoleCmd.Flags().String("required", "", "path to the role definition vAssessment requires (required)")
	_ = validateRoleCmd.MarkFlagRequired("granted")
	_ = validateRoleCmd.MarkFlagRequired("required")
	validateCmd.AddCommand(validateRoleCmd)
}
