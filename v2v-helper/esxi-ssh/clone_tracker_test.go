// Copyright © 2024 The vjailbreak authors

package esxissh

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// fakeChecker stands in for the SSH client so completion logic can be tested
// without touching a real ESXi host. logsByCall lets a test vary the log content
// across polls; the last entry is repeated once exhausted.
type fakeChecker struct {
	visible    bool
	logs       string
	logsByCall []string
	calls      int
	lastCmd    string
}

func (f *fakeChecker) CheckCloneStatus(int) (bool, error) { return f.visible, nil }

func (f *fakeChecker) ExecuteCommand(cmd string) (string, error) {
	f.lastCmd = cmd
	f.calls++
	if len(f.logsByCall) == 0 {
		return f.logs, nil
	}
	if f.calls > len(f.logsByCall) {
		return f.logsByCall[len(f.logsByCall)-1], nil
	}
	return f.logsByCall[f.calls-1], nil
}

func intPtr(i int) *int { return &i }

func newTestTracker(c cloneProcessChecker) *CloneTracker {
	ct := NewCloneTracker(c, &VmkfstoolsTask{Pid: 123, LogFile: "/tmp/clone.log"}, 0, nil)
	ct.SetPollInterval(time.Millisecond)
	return ct
}

func TestWrapWithExitSentinel(t *testing.T) {
	got := wrapWithExitSentinel("vmkfstools -i src -d rdm:dev desc", "/tmp/clone.log")
	want := `( vmkfstools -i src -d rdm:dev desc; echo "VJB_EXIT:$?" ) >/tmp/clone.log 2>&1 & echo $!`
	if got != want {
		t.Errorf("wrapWithExitSentinel()\n got: %s\nwant: %s", got, want)
	}
}

func TestParseExitCode(t *testing.T) {
	tests := []struct {
		name string
		log  string
		want *int
	}{
		{name: "no sentinel yet", log: "Clone: 42% done.", want: nil},
		{name: "success", log: "Clone: 100% done.\nVJB_EXIT:0", want: intPtr(0)},
		{name: "failure", log: "Usage: vmkfstools ...\nVJB_EXIT:1", want: intPtr(1)},
		{name: "command not found", log: "not found\nVJB_EXIT:127", want: intPtr(127)},
		{name: "killed by signal", log: "Killed\nVJB_EXIT:137", want: intPtr(137)},
		{name: "partially flushed sentinel is ignored", log: "Clone: 3% done.\nVJB_EXIT:", want: nil},
		{name: "sentinel-like text mid-line is not matched", log: "note VJB_EXIT:0 embedded", want: nil},
		{name: "last sentinel wins", log: "VJB_EXIT:1\nVJB_EXIT:0", want: intPtr(0)},
		{name: "empty log", log: "", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertExitCode(t, parseExitCode(tt.log), tt.want)
		})
	}
}

// TestParseExitCodeAfterCarriageReturnProgress is the regression test for the
// sentinel being missed when vmkfstools is killed mid-copy. Progress updates are
// separated by \r with no trailing newline, so the sentinel lands on the same
// line and Go's (?m)^ — which only matches after \n — never sees it unless the
// input is normalised first.
func TestParseExitCodeAfterCarriageReturnProgress(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want *int
	}{
		{
			name: "killed mid-progress",
			raw:  "Clone: 0% done.\rClone: 37% done.\rVJB_EXIT:137",
			want: intPtr(137),
		},
		{
			name: "clean completion with no trailing newline",
			raw:  "Clone: 99% done.\rClone: 100% done.\rVJB_EXIT:0",
			want: intPtr(0),
		},
		{
			name: "CRLF line endings",
			raw:  "Clone: 100% done.\r\nVJB_EXIT:0",
			want: intPtr(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized := strings.ReplaceAll(tt.raw, "\r", "\n")
			assertExitCode(t, parseExitCode(normalized), tt.want)

			// And end to end through classify, which owns the normalisation.
			ct := newTestTracker(&fakeChecker{})
			status := ct.classify(tt.raw, false, time.Now())
			if status.ExitCode == nil {
				t.Fatalf("classify did not normalise carriage returns; sentinel lost in %q", tt.raw)
			}
			assertExitCode(t, status.ExitCode, tt.want)
		})
	}
}

func assertExitCode(t *testing.T, got, want *int) {
	t.Helper()
	switch {
	case want == nil && got != nil:
		t.Errorf("expected nil exit code, got %d", *got)
	case want != nil && got == nil:
		t.Errorf("expected exit code %d, got nil", *want)
	case want != nil && *got != *want:
		t.Errorf("exit code = %d, want %d", *got, *want)
	}
}

func TestParseProgress(t *testing.T) {
	tests := []struct {
		name string
		log  string
		want float64
	}{
		{name: "empty", log: "", want: 0},
		{name: "single", log: "Clone: 7% done.", want: 7},
		{name: "highest wins", log: "Clone: 0% done.\nClone: 55% done.\nClone: 12% done.", want: 55},
		{name: "complete", log: "Clone: 100% done.", want: 100},
		{name: "no period", log: "Clone: 33% done", want: 33},
		{name: "ignores sentinel", log: "Clone: 100% done.\nVJB_EXIT:0", want: 100},
		{name: "unrelated output", log: "some error text", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseProgress(tt.log); got != tt.want {
				t.Errorf("parseProgress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogTail(t *testing.T) {
	t.Run("collapses progress spam and sentinel", func(t *testing.T) {
		var sb strings.Builder
		for i := 0; i <= 100; i++ {
			fmt.Fprintf(&sb, "Clone: %d%% done.\n", i)
		}
		sb.WriteString("DISKLIB-LIB : Failed to open source\nVJB_EXIT:1")

		got := logTail(sb.String(), 20)
		if strings.Contains(got, "Clone:") || strings.Contains(got, exitSentinelPrefix) {
			t.Errorf("progress and sentinel lines should be filtered, got:\n%s", got)
		}
		if !strings.Contains(got, "Failed to open source") {
			t.Errorf("the useful line was dropped, got:\n%s", got)
		}
	})

	t.Run("limits to n lines and keeps the tail", func(t *testing.T) {
		lines := make([]string, 50)
		for i := range lines {
			lines[i] = fmt.Sprintf("line %d", i)
		}
		got := logTail(strings.Join(lines, "\n"), 5)
		if n := len(strings.Split(got, "\n")); n != 5 {
			t.Errorf("expected 5 lines, got %d", n)
		}
		if !strings.Contains(got, "line 49") {
			t.Error("expected the tail, not the head")
		}
	})

	t.Run("empty", func(t *testing.T) {
		if got := logTail("", 20); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name        string
		log         string
		visible     bool
		elapsed     time.Duration
		goneSince   time.Duration // how long the process has been absent, 0 = not yet seen absent
		wantDone    bool
		wantErr     bool
		wantPercent float64
		wantExit    *int
	}{
		{
			name:        "in progress",
			log:         "Clone: 42% done.",
			visible:     true,
			wantDone:    false,
			wantPercent: 42,
		},
		{
			name:        "exit 0 is success",
			log:         "Clone: 100% done.\nVJB_EXIT:0",
			wantDone:    true,
			wantErr:     false,
			wantPercent: 100,
			wantExit:    intPtr(0),
		},
		{
			name:        "exit 0 wins even while the process is briefly still visible",
			log:         "Clone: 100% done.\nVJB_EXIT:0",
			visible:     true,
			wantDone:    true,
			wantPercent: 100,
			wantExit:    intPtr(0),
		},
		{
			name:     "non-zero exit is a failure",
			log:      "Usage: vmkfstools\nVJB_EXIT:1",
			wantDone: true,
			wantErr:  true,
			wantExit: intPtr(1),
		},
		{
			name:        "regression #2270: died at 0% with no sentinel is a failure, not success",
			log:         "Usage: vmkfstools -i ...",
			elapsed:     10 * time.Minute,
			goneSince:   2 * time.Minute,
			wantDone:    true,
			wantErr:     true,
			wantPercent: 0,
		},
		{
			name:     "empty log within startup window is still starting",
			log:      "",
			elapsed:  3 * time.Second,
			wantDone: false,
		},
		{
			name:      "empty log past startup window fails",
			log:       "",
			elapsed:   10 * time.Minute,
			goneSince: 2 * time.Minute,
			wantDone:  true,
			wantErr:   true,
		},
		{
			name:        "process gone but inside the grace period keeps waiting",
			log:         "Clone: 50% done.",
			elapsed:     time.Minute,
			goneSince:   time.Second,
			wantDone:    false,
			wantPercent: 50,
		},
		{
			name:        "100% without a sentinel is NOT success",
			log:         "Clone: 100% done.",
			elapsed:     time.Minute,
			goneSince:   2 * time.Minute,
			wantDone:    true,
			wantErr:     true,
			wantPercent: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			ct := newTestTracker(&fakeChecker{})
			ct.startTime = now.Add(-tt.elapsed)
			if tt.goneSince > 0 {
				ct.processGoneSince = now.Add(-tt.goneSince)
			}

			status := ct.classify(tt.log, tt.visible, now)

			if status.Done != tt.wantDone {
				t.Errorf("Done = %v, want %v (err: %v)", status.Done, tt.wantDone, status.Err)
			}
			if gotErr := status.Err != nil; gotErr != tt.wantErr {
				t.Errorf("error present = %v, want %v (err: %v)", gotErr, tt.wantErr, status.Err)
			}
			if status.PercentDone != tt.wantPercent {
				t.Errorf("PercentDone = %v, want %v", status.PercentDone, tt.wantPercent)
			}
			assertExitCode(t, status.ExitCode, tt.wantExit)
		})
	}
}

// TestClassifyErrorsNameTheCause guards against a failure being reported through
// the wrong branch, which the Done/Err booleans alone cannot distinguish.
func TestClassifyErrorsNameTheCause(t *testing.T) {
	now := time.Now()

	ct := newTestTracker(&fakeChecker{})
	status := ct.classify("DISKLIB-LIB : Failed to open source\nVJB_EXIT:1", false, now)
	if status.Err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"status 1", "Failed to open source"} {
		if !strings.Contains(status.Err.Error(), want) {
			t.Errorf("error missing %q:\n%v", want, status.Err)
		}
	}

	ct = newTestTracker(&fakeChecker{})
	ct.startTime = now.Add(-10 * time.Minute)
	ct.processGoneSince = now.Add(-2 * time.Minute)
	status = ct.classify("Clone: 63% done.", false, now)
	if status.Err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(status.Err.Error(), "without reporting an exit status") {
		t.Errorf("expected the no-sentinel message, got: %v", status.Err)
	}
	if !strings.Contains(status.Err.Error(), "63%") {
		t.Errorf("error should report progress reached, got: %v", status.Err)
	}
}

// TestWaitForCompletionRejectsUnverifiableClone covers the case where no log file
// was recorded, so completion cannot be checked at all.
func TestWaitForCompletionRejectsUnverifiableClone(t *testing.T) {
	ct := NewCloneTracker(&fakeChecker{}, &VmkfstoolsTask{Pid: 1}, 0, nil)
	err := ct.WaitForCompletion(context.Background())
	if err == nil {
		t.Fatal("expected an error when no log file is available")
	}
	if !strings.Contains(err.Error(), "cannot be verified") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestWaitForCompletionFailsFastOnEarlyExit reproduces the customer failure end
// to end: vmkfstools dies immediately, and the tracker must report an error
// rather than "completed successfully in 2s".
func TestWaitForCompletionFailsFastOnEarlyExit(t *testing.T) {
	ct := newTestTracker(&fakeChecker{
		visible: false,
		logs:    "Usage: vmkfstools -i <source> ...\nVJB_EXIT:1",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ct.WaitForCompletion(ctx)
	if err == nil {
		t.Fatal("clone that exited with status 1 was reported as successful (issue #2270)")
	}
	if !strings.Contains(err.Error(), "status 1") {
		t.Errorf("error should name the exit status, got: %v", err)
	}
}

// TestWaitForCompletionToleratesLateSentinel covers the window where the process
// has exited but its final output has not yet appeared in the log. The grace
// period must keep the tracker waiting rather than declaring a spurious failure.
// The pre-sentinel log deliberately sits below 100% so the grace path — and not
// a progress-based shortcut — is what is under test.
func TestWaitForCompletionToleratesLateSentinel(t *testing.T) {
	checker := &fakeChecker{
		visible: false,
		logsByCall: []string{
			"Clone: 50% done.",
			"Clone: 50% done.",
			"Clone: 50% done.\nVJB_EXIT:0",
		},
	}
	ct := newTestTracker(checker)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := ct.WaitForCompletion(ctx); err != nil {
		t.Fatalf("a late sentinel should not fail the clone, got: %v", err)
	}
	if checker.calls < 3 {
		t.Errorf("expected the tracker to keep polling for the sentinel, got %d polls", checker.calls)
	}
}

// TestWaitForCompletionSucceedsOnCleanClone is the happy path.
func TestWaitForCompletionSucceedsOnCleanClone(t *testing.T) {
	ct := newTestTracker(&fakeChecker{
		visible: false,
		logs:    "Clone: 0% done.\rClone: 100% done.\rVJB_EXIT:0",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := ct.WaitForCompletion(ctx); err != nil {
		t.Fatalf("clean clone should succeed, got: %v", err)
	}
}

// TestTrackerReadsTheTaskLogFile pins that the tracker reads the log belonging to
// its own task, which matters because clone logs are per-operation.
func TestTrackerReadsTheTaskLogFile(t *testing.T) {
	checker := &fakeChecker{visible: true, logs: "Clone: 1% done."}
	ct := NewCloneTracker(checker, &VmkfstoolsTask{Pid: 7, LogFile: "/tmp/specific.log"}, 2, nil)
	ct.GetStatus()

	if !strings.Contains(checker.lastCmd, "/tmp/specific.log") {
		t.Errorf("tracker read the wrong log file: %q", checker.lastCmd)
	}
}

// TestProgressIsReportedInFivePercentBuckets covers the operator-facing output.
func TestProgressIsReportedInFivePercentBuckets(t *testing.T) {
	var got []string
	ct := NewCloneTracker(&fakeChecker{}, &VmkfstoolsTask{LogFile: "/tmp/x.log"}, 1, loggerFunc(func(m string) {
		got = append(got, m)
	}))

	for _, pct := range []float64{0, 3, 5, 7, 12, 12, 100} {
		ct.logProgressIfNeeded(pct)
	}

	want := []string{
		"Copying disk 1, Completed: 0%",
		"Copying disk 1, Completed: 5%",
		"Copying disk 1, Completed: 10%",
		"Copying disk 1, Completed: 100%",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d messages %q, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("message %d = %q, want %q", i, got[i], want[i])
		}
	}
}

type loggerFunc func(string)

func (f loggerFunc) LogMessage(msg string) { f(msg) }
