package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/platform9/vjailbreak/vassessment/version"
)

func TestVersionCommand_PrintsVersion(t *testing.T) {
	out := &bytes.Buffer{}
	versionCmd.SetOut(out)
	versionCmd.Run(versionCmd, nil)

	got := out.String()
	want := "vassessment version " + version.Version + "\n"
	if got != want {
		t.Fatalf("versionCmd output = %q, want %q", got, want)
	}
}

func TestVersionCommand_Registered(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Use == "version" {
			return
		}
	}
	t.Fatal("version command not registered on rootCmd")
}

func TestVersion_DefaultsToDev(t *testing.T) {
	if !strings.EqualFold(version.Version, "dev") {
		t.Skipf("version.Version overridden at build time to %q", version.Version)
	}
}
