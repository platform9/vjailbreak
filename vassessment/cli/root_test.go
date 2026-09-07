package cli

import "testing"

func TestExecute_NoArgsSucceeds(t *testing.T) {
	rootCmd.SetArgs([]string{})
	if err := Execute(); err != nil {
		t.Fatalf("Execute() with no args returned error: %v", err)
	}
}

func TestExecute_UnknownSubcommandErrors(t *testing.T) {
	rootCmd.SetArgs([]string{"does-not-exist"})
	defer rootCmd.SetArgs([]string{})

	if err := Execute(); err == nil {
		t.Fatal("Execute() with an unknown subcommand should return an error")
	}
}
