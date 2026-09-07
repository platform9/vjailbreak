package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeRoleFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("writing fixture %s: %v", path, err)
	}
	return path
}

func TestValidateRoleCommand(t *testing.T) {
	dir := t.TempDir()
	granted := writeRoleFile(t, dir, "granted.json", `{"name":"vassessment-readonly","privileges":["System.View","System.Read"]}`)
	requiredOK := writeRoleFile(t, dir, "required-ok.json", `{"name":"required","privileges":["System.View"]}`)
	requiredMissing := writeRoleFile(t, dir, "required-missing.json", `{"name":"required","privileges":["System.View","Datastore.Browse"]}`)

	cases := []struct {
		name     string
		required string
		wantErr  bool
	}{
		{"all required privileges granted", requiredOK, false},
		{"missing a required privilege", requiredMissing, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			rootCmd.SetOut(out)
			rootCmd.SetErr(out)
			rootCmd.SetArgs([]string{"validate", "role", "--granted=" + granted, "--required=" + c.required})
			defer rootCmd.SetArgs([]string{})

			err := rootCmd.Execute()
			if (err != nil) != c.wantErr {
				t.Fatalf("Execute() error = %v, wantErr %v (output: %s)", err, c.wantErr, out.String())
			}
		})
	}
}
