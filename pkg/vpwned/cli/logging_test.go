package cli

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// captureLog points the standard logger at a buffer for the duration of a test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	originalWriter, originalFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
	})

	return &buf
}

// An API server warning is one sentence and should print as one sentence. logr's funcr
// renderer would emit `"level"=0 "msg"="..."`, which reads like debug output.
func TestStdLogSinkRendersOneReadableLine(t *testing.T) {
	buf := captureLog(t)

	NewStdLogSink().Info("spec.template.metadata.annotations[...]: deprecated since v1.30")

	got := strings.TrimSpace(buf.String())
	want := "spec.template.metadata.annotations[...]: deprecated since v1.30"
	if got != want {
		t.Errorf("log line = %q, want exactly %q", got, want)
	}
}

func TestStdLogSinkFormatting(t *testing.T) {
	tests := []struct {
		name string
		emit func(buf *bytes.Buffer)
		want string
	}{
		{
			name: "key/value pairs are appended",
			emit: func(*bytes.Buffer) {
				NewStdLogSink().Info("applied", "kind", "Deployment", "code", 299)
			},
			want: "applied kind=Deployment code=299",
		},
		{
			name: "values attached with WithValues are carried",
			emit: func(*bytes.Buffer) {
				NewStdLogSink().WithValues("tag", "v0.4.9").Info("upgrading")
			},
			want: "upgrading tag=v0.4.9",
		},
		{
			name: "a name is shown as a prefix",
			emit: func(*bytes.Buffer) {
				NewStdLogSink().WithName("upgrade").Info("started")
			},
			want: "[upgrade] started",
		},
		{
			name: "nested names are joined",
			emit: func(*bytes.Buffer) {
				NewStdLogSink().WithName("upgrade").WithName("crds").Info("applied")
			},
			want: "[upgrade.crds] applied",
		},
		{
			name: "errors carry the cause",
			emit: func(*bytes.Buffer) {
				NewStdLogSink().Error(errors.New("boom"), "apply failed")
			},
			want: "error: boom: apply failed",
		},
		{
			name: "a nil error still reports as an error",
			emit: func(*bytes.Buffer) {
				NewStdLogSink().Error(nil, "apply failed")
			},
			want: "error: apply failed",
		},
		{
			name: "an odd number of key/values is reported, not dropped",
			emit: func(*bytes.Buffer) {
				NewStdLogSink().Info("applied", "kind")
			},
			want: "applied kind=<missing>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureLog(t)

			tt.emit(buf)

			if got := strings.TrimSpace(buf.String()); got != tt.want {
				t.Errorf("log line = %q, want %q", got, tt.want)
			}
		})
	}
}

// WithValues must not mutate the logger it was derived from.
func TestStdLogSinkWithValuesDoesNotLeak(t *testing.T) {
	buf := captureLog(t)

	base := NewStdLogSink()
	base.WithValues("tag", "v0.4.9")
	base.Info("no values here")

	if got := strings.TrimSpace(buf.String()); got != "no values here" {
		t.Errorf("log line = %q, want the base logger to be unchanged", got)
	}
}

// Without an installed logger, controller-runtime prints a "log.SetLogger(...) was never
// called" notice with a stack trace and discards the message - which for these binaries is
// the API server's Warning header on the requests the upgrade makes.
func TestSetupControllerRuntimeLoggerRoutesToStandardLogger(t *testing.T) {
	buf := captureLog(t)

	SetupControllerRuntimeLogger()

	logf.Log.Info("api server warning", "code", 299)

	got := buf.String()
	if !strings.Contains(got, "api server warning") {
		t.Errorf("output = %q, want it to contain the message", got)
	}
	if !strings.Contains(got, "code=299") {
		t.Errorf("output = %q, want it to carry the key/value pairs", got)
	}
	if strings.Contains(got, "was never called") {
		t.Error("the missing-logger notice was still printed after a logger was installed")
	}
	if strings.Contains(got, `"msg"=`) {
		t.Errorf("output = %q, want a plain line rather than logr's key/value rendering", got)
	}
}
