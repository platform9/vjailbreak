package cli

import (
	"bytes"
	"log"
	"strings"
	"testing"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// Without an installed logger, controller-runtime prints a one-off
// "log.SetLogger(...) was never called" notice with a stack trace and discards the
// message - which for the upgrade job is the API server's Warning header on its
// server-side apply requests.
func TestSetupControllerRuntimeLoggerRoutesToStandardLogger(t *testing.T) {
	var buf bytes.Buffer
	originalWriter, originalFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
	})

	setupControllerRuntimeLogger()

	logf.Log.Info("api server warning", "code", 299)

	got := buf.String()
	if !strings.Contains(got, "api server warning") {
		t.Errorf("controller-runtime log output = %q, want it to contain the message", got)
	}
	if !strings.Contains(got, "299") {
		t.Errorf("controller-runtime log output = %q, want it to carry the key/value pairs", got)
	}
	if strings.Contains(got, "was never called") {
		t.Error("the missing-logger notice was still printed after a logger was installed")
	}
}
