package cli

import (
	"bytes"
	"testing"
)

func TestValidateConnectionCommand(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"valid input", []string{"validate", "connection", "--host=vcenter01.acme.internal", "--username=svc", "--password=secret"}, false},
		{"missing password", []string{"validate", "connection", "--host=vcenter01.acme.internal", "--username=svc"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// pflag's FlagSet keeps a flag's last value if a later Execute()
			// doesn't pass it again, so reset before every subtest.
			for _, f := range []string{"host", "username", "password"} {
				if err := validateConnectionCmd.Flags().Set(f, ""); err != nil {
					t.Fatalf("resetting --%s: %v", f, err)
				}
			}

			out := &bytes.Buffer{}
			rootCmd.SetOut(out)
			rootCmd.SetErr(out)
			rootCmd.SetArgs(c.args)
			defer rootCmd.SetArgs([]string{})

			err := rootCmd.Execute()
			if (err != nil) != c.wantErr {
				t.Fatalf("Execute(%v) error = %v, wantErr %v (output: %s)", c.args, err, c.wantErr, out.String())
			}
		})
	}
}
