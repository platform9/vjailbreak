// Copyright © 2024 The vjailbreak authors

package esxissh

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ProgressLogger is an interface for logging clone progress messages.
// This allows the migrate package to receive progress updates without circular imports.
type ProgressLogger interface {
	LogMessage(msg string)
}

// cloneProcessChecker is the subset of Client the tracker depends on, so the
// completion logic can be unit tested without an SSH connection.
type cloneProcessChecker interface {
	CheckCloneStatus(pid int) (bool, error)
	ExecuteCommand(command string) (string, error)
}

const (
	// exitSentinelPrefix marks the line the wrapper subshell appends to the clone
	// log once vmkfstools returns, carrying its exit status. Without it the
	// tracker cannot distinguish "finished" from "died before doing anything".
	exitSentinelPrefix = "VJB_EXIT:"

	// startupGrace is how long an empty log is tolerated before the clone is
	// expected to have produced output.
	startupGrace = 5 * time.Minute

	// completionGrace is how long the tracker keeps polling after the process
	// disappears without having written an exit sentinel.
	//
	// Because success requires positive evidence, a missing sentinel is treated
	// as failure, so benign causes must not be mistaken for one. Liveness is
	// probed before the log is read, which removes the main race, but output
	// flushing and SSH latency can still leave a brief window. A genuine failure
	// (wrapper killed before it could report, host rebooted, log removed)
	// persists past this window.
	completionGrace = 30 * time.Second

	// logTailLines is how much of the vmkfstools log is attached to errors.
	logTailLines = 20
)

// exitSentinelRe matches a complete sentinel line. Anchoring to end-of-line
// avoids reading a partially flushed value.
var exitSentinelRe = regexp.MustCompile(`(?m)^` + exitSentinelPrefix + `(-?\d+)[ \t]*$`)

// wrapWithExitSentinel runs cmd in the background with stdout and stderr going to
// logFile, and appends the exit status once it returns. The echoed PID is the
// wrapping subshell, which lives exactly as long as cmd.
func wrapWithExitSentinel(cmd, logFile string) string {
	return fmt.Sprintf("( %s; echo \"%s$?\" ) >%s 2>&1 & echo $!",
		cmd, exitSentinelPrefix, shellQuote(logFile))
}

// CloneStatus is a point-in-time observation of a clone operation.
type CloneStatus struct {
	// Done reports whether the clone has reached a terminal state. While false,
	// the caller should keep polling.
	Done bool
	// Err is set only when the clone reached a terminal state unsuccessfully.
	Err error
	// PercentDone is the highest progress seen in the log so far.
	PercentDone float64
	// ExitCode is vmkfstools' exit status, or nil if it has not reported yet.
	ExitCode    *int
	ElapsedTime time.Duration
}

// CloneTracker monitors a vmkfstools clone operation in real-time
type CloneTracker struct {
	client            cloneProcessChecker
	task              *VmkfstoolsTask
	startTime         time.Time
	pollInterval      time.Duration
	lastLoggedPercent int
	processGoneSince  time.Time
	logger            ProgressLogger
	diskIndex         int
}

// NewCloneTracker creates a new clone operation tracker.
// logger can be nil if no progress events are needed.
func NewCloneTracker(client cloneProcessChecker, task *VmkfstoolsTask, diskIndex int, logger ProgressLogger) *CloneTracker {
	return &CloneTracker{
		client:            client,
		task:              task,
		startTime:         time.Now(),
		pollInterval:      10 * time.Second,
		lastLoggedPercent: -1,
		logger:            logger,
		diskIndex:         diskIndex,
	}
}

// SetPollInterval sets how often to check clone status
func (ct *CloneTracker) SetPollInterval(interval time.Duration) {
	ct.pollInterval = interval
}

// GetStatus returns the current status of the clone operation
func (ct *CloneTracker) GetStatus() *CloneStatus {
	// Order matters: check liveness before reading the log. The sentinel is
	// written before the wrapper subshell exits, so a log read *after* observing
	// the process is gone is guaranteed to contain it. Reading the log first
	// would sample it before the sentinel was written and then observe the exit,
	// making a healthy clone look like it vanished without reporting.
	processVisible, _ := ct.client.CheckCloneStatus(ct.task.Pid)
	logContent := ct.readLogFile()

	status := ct.classify(logContent, processVisible, time.Now())
	ct.logProgressIfNeeded(status.PercentDone)

	return status
}

// readLogFile reads the vmkfstools log file content
func (ct *CloneTracker) readLogFile() string {
	logCmd := fmt.Sprintf("cat %s 2>/dev/null", shellQuote(ct.task.LogFile))
	content, _ := ct.client.ExecuteCommand(logCmd)
	return strings.TrimSpace(content)
}

// classify turns the observable signals into a terminal or non-terminal status.
//
// The governing rule is that success requires positive evidence: a zero exit
// status from vmkfstools. Previously any state that was not recognisably
// "running" was reported as successful completion, so a process that died at 0%
// was indistinguishable from one that finished (issue #2270). Every path that
// is not a confirmed success is now an error.
func (ct *CloneTracker) classify(logContent string, processVisible bool, now time.Time) *CloneStatus {
	// vmkfstools separates progress updates with \r rather than \n, so the
	// sentinel can end up on the same line as the last progress update when the
	// clone is killed mid-copy. Normalising first is what makes it visible.
	normalized := strings.ReplaceAll(logContent, "\r", "\n")

	status := &CloneStatus{
		ElapsedTime: now.Sub(ct.startTime),
		PercentDone: parseProgress(normalized),
		ExitCode:    parseExitCode(normalized),
	}

	// The wrapper reported vmkfstools' exit status: authoritative either way.
	if status.ExitCode != nil {
		status.Done = true
		if *status.ExitCode != 0 {
			status.Err = fmt.Errorf("clone failed for disk %d after %v: vmkfstools exited with status %d at %.0f%%%s",
				ct.diskIndex, status.ElapsedTime.Round(time.Second), *status.ExitCode,
				status.PercentDone, formatLogTail(logTail(normalized, logTailLines)))
		} else if status.PercentDone < 100 {
			log.Printf("WARNING: disk %d clone exited successfully but the log only reached %.0f%%",
				ct.diskIndex, status.PercentDone)
		}
		return status
	}

	if processVisible {
		ct.processGoneSince = time.Time{}
		return status
	}

	// Process is not visible and has not reported an exit status.

	// Nothing in the log yet and still within startup: the process may not have
	// been scheduled, or output has not been flushed.
	if normalized == "" && status.ElapsedTime < startupGrace {
		return status
	}

	// Allow a short grace period for the sentinel to land before giving up.
	if ct.processGoneSince.IsZero() {
		ct.processGoneSince = now
	}
	if now.Sub(ct.processGoneSince) < completionGrace {
		return status
	}

	status.Done = true
	status.Err = fmt.Errorf("clone for disk %d stopped at %.0f%% after %v without reporting an exit status%s",
		ct.diskIndex, status.PercentDone, status.ElapsedTime.Round(time.Second),
		formatLogTail(logTail(normalized, logTailLines)))
	return status
}

// parseProgress extracts the highest percentage from log content, which must
// already have carriage returns normalised to newlines.
func parseProgress(normalized string) float64 {
	var maxPct float64

	for _, line := range strings.Split(normalized, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var pct float64
		// Try with period at end (most common format)
		if _, err := fmt.Sscanf(line, "Clone: %f%% done.", &pct); err == nil {
			if pct > maxPct {
				maxPct = pct
			}
			continue
		}
		// Try without period
		if _, err := fmt.Sscanf(line, "Clone: %f%% done", &pct); err == nil {
			if pct > maxPct {
				maxPct = pct
			}
		}
	}

	return maxPct
}

// parseExitCode returns vmkfstools' exit status from the sentinel line the
// wrapper appends, or nil if it has not been written yet. Input must already
// have carriage returns normalised. The last complete sentinel wins.
func parseExitCode(normalized string) *int {
	matches := exitSentinelRe.FindAllStringSubmatch(normalized, -1)
	if len(matches) == 0 {
		return nil
	}
	code, err := strconv.Atoi(matches[len(matches)-1][1])
	if err != nil {
		return nil
	}
	return &code
}

// logTail returns the last n lines of the log, with progress spam collapsed so
// the useful output is not buried under thousands of carriage-return updates.
func logTail(normalized string, n int) string {
	kept := make([]string, 0, n)
	for _, line := range strings.Split(normalized, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Clone: ") || strings.HasPrefix(line, exitSentinelPrefix) {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) > n {
		kept = kept[len(kept)-n:]
	}
	return strings.Join(kept, "\n")
}

func formatLogTail(tail string) string {
	if tail == "" {
		return " (vmkfstools produced no output)"
	}
	return "\nvmkfstools output:\n" + tail
}

// logProgressIfNeeded logs progress at 5% increments
func (ct *CloneTracker) logProgressIfNeeded(percentDone float64) {
	currentBucket := (int(percentDone) / 5) * 5
	if currentBucket > ct.lastLoggedPercent {
		msg := fmt.Sprintf("Copying disk %d, Completed: %d%%", ct.diskIndex, currentBucket)
		log.Print(msg)
		if ct.logger != nil {
			ct.logger.LogMessage(msg)
		}
		ct.lastLoggedPercent = currentBucket
	}
}

// WaitForCompletion blocks until the clone completes, fails, or context is
// cancelled. It returns an error unless the clone produced positive evidence of
// success.
func (ct *CloneTracker) WaitForCompletion(ctx context.Context) error {
	msg := fmt.Sprintf("Starting clone monitor for disk %d (PID %d)", ct.diskIndex, ct.task.Pid)
	log.Print(msg)
	if ct.logger != nil {
		ct.logger.LogMessage(msg)
	}

	if ct.task.LogFile == "" {
		return fmt.Errorf("cannot monitor clone for disk %d: no log file was recorded, "+
			"so completion cannot be verified", ct.diskIndex)
	}

	ticker := time.NewTicker(ct.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Clone monitoring cancelled")
			return ctx.Err()

		case <-ticker.C:
			status := ct.GetStatus()
			if !status.Done {
				continue
			}
			if status.Err != nil {
				return status.Err
			}

			msg := fmt.Sprintf("Disk %d clone completed successfully in %v",
				ct.diskIndex, status.ElapsedTime.Round(time.Second))
			log.Print(msg)
			if ct.logger != nil {
				ct.logger.LogMessage(msg)
			}
			return nil
		}
	}
}
