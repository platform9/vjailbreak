package cli

import (
	"fmt"
	"log"
	"strings"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// stdLogSink renders controller-runtime's logs as one readable line through the standard
// logger the vpwned binaries already write to.
//
// The alternative, logr's funcr, renders its own key/value form - `"level"=0 "msg"="..."` -
// which turns a one-sentence API server warning into something that reads like debug
// output. Almost everything controller-runtime emits here is a Warning header (HTTP 299)
// returned on the requests the upgrade makes, so it is worth printing plainly.
type stdLogSink struct {
	name   string
	values []any
}

// NewStdLogSink returns a logr.Logger that writes through the standard log package.
func NewStdLogSink() logr.Logger {
	return logr.New(stdLogSink{})
}

// SetupControllerRuntimeLogger installs that logger as controller-runtime's.
//
// controller-runtime only logs through a logger the process installs. With none installed
// it prints a one-off "log.SetLogger(...) was never called; logs will not be displayed"
// notice with a full stack trace and drops the message itself, so the warnings the API
// server returns are both noisy and unreadable.
func SetupControllerRuntimeLogger() {
	logf.SetLogger(NewStdLogSink())
}

func (s stdLogSink) Init(logr.RuntimeInfo) {}

// Enabled reports true for every level: controller-runtime only uses this logger for
// warnings and errors here, and dropping them is what caused the original problem.
func (s stdLogSink) Enabled(int) bool { return true }

func (s stdLogSink) Info(_ int, msg string, kvs ...any) {
	log.Print(s.render("", msg, kvs))
}

func (s stdLogSink) Error(err error, msg string, kvs ...any) {
	prefix := "error"
	if err != nil {
		prefix = fmt.Sprintf("error: %v", err)
	}
	log.Print(s.render(prefix, msg, kvs))
}

func (s stdLogSink) WithValues(kvs ...any) logr.LogSink {
	return stdLogSink{name: s.name, values: append(append([]any{}, s.values...), kvs...)}
}

func (s stdLogSink) WithName(name string) logr.LogSink {
	if s.name != "" {
		name = s.name + "." + name
	}
	return stdLogSink{name: name, values: s.values}
}

// render builds "[name] prefix: message key=value" and omits whatever is absent, so a bare
// warning prints as just the warning.
func (s stdLogSink) render(prefix, msg string, kvs []any) string {
	var b strings.Builder

	if s.name != "" {
		fmt.Fprintf(&b, "[%s] ", s.name)
	}
	if prefix != "" {
		fmt.Fprintf(&b, "%s: ", prefix)
	}
	b.WriteString(msg)

	for _, kv := range formatKeysAndValues(append(append([]any{}, s.values...), kvs...)) {
		fmt.Fprintf(&b, " %s", kv)
	}

	return b.String()
}

// formatKeysAndValues pairs up the key/value slice logr hands over. A trailing key with no
// value is reported rather than dropped, since silently losing context is what this whole
// file exists to avoid.
func formatKeysAndValues(kvs []any) []string {
	pairs := make([]string, 0, (len(kvs)+1)/2)

	for i := 0; i < len(kvs); i += 2 {
		if i+1 >= len(kvs) {
			pairs = append(pairs, fmt.Sprintf("%v=<missing>", kvs[i]))
			break
		}
		pairs = append(pairs, fmt.Sprintf("%v=%v", kvs[i], kvs[i+1]))
	}

	return pairs
}
