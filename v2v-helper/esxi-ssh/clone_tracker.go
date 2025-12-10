// Copyright © 2024 The vjailbreak authors

package esxissh

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// CloneStatus represents the current state of a clone operation
type CloneStatus struct {
	PID         int
	IsRunning   bool
	PercentDone float64
	ElapsedTime time.Duration
	Error       string
}

// CloneTracker monitors a vmkfstools clone operation in real-time
type CloneTracker struct {
	client            *Client
	task              *VmkfstoolsTask
	startTime         time.Time
	pollInterval      time.Duration
	startupTimeout    time.Duration
	lastLoggedPercent int
}

// NewCloneTracker creates a new clone operation tracker
func NewCloneTracker(client *Client, task *VmkfstoolsTask) *CloneTracker {
	return &CloneTracker{
		client:            client,
		task:              task,
		startTime:         time.Now(),
		pollInterval:      2 * time.Second,
		startupTimeout:    5 * time.Minute,
		lastLoggedPercent: -1,
	}
}

// SetPollInterval sets how often to check clone status
func (ct *CloneTracker) SetPollInterval(interval time.Duration) {
	ct.pollInterval = interval
}

// SetStartupTimeout sets how long to wait for the process to actually start executing
func (ct *CloneTracker) SetStartupTimeout(timeout time.Duration) {
	ct.startupTimeout = timeout
}

// GetStatus returns the current status of the clone operation
func (ct *CloneTracker) GetStatus(ctx context.Context) (*CloneStatus, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	status := &CloneStatus{
		PID:         ct.task.Pid,
		ElapsedTime: time.Since(ct.startTime),
	}

	// Read log file content
	logContent := ct.readLogFile()

	// Parse state from log content
	status.PercentDone = ct.parseProgress(logContent)
	status.Error = ct.parseError(logContent)
	status.IsRunning = ct.determineIfRunning(logContent, status.PercentDone, status.Error)

	klog.Infof("GetStatus: PID=%d, progress=%.0f%%, isRunning=%v, error=%q, elapsed=%v",
		status.PID, status.PercentDone, status.IsRunning, status.Error, status.ElapsedTime.Round(time.Second))

	// Log progress at 5% increments
	ct.logProgressIfNeeded(status.PercentDone)

	return status, nil
}

// readLogFile reads the vmkfstools log file content
func (ct *CloneTracker) readLogFile() string {
	if ct.task.LogFile == "" {
		klog.Warning("readLogFile: LogFile path is empty")
		return ""
	}
	logCmd := fmt.Sprintf("cat %s 2>/dev/null", ct.task.LogFile)
	content, err := ct.client.ExecuteCommand(logCmd)
	if err != nil {
		klog.Warningf("readLogFile: Error reading log file %s: %v", ct.task.LogFile, err)
	}
	content = strings.TrimSpace(content)

	// Log raw content for debugging (first 300 chars)
	if len(content) > 0 {
		preview := content
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		// Show raw bytes to detect \r characters
		klog.Infof("readLogFile: content length=%d, raw preview: %q", len(content), preview)
	} else {
		klog.Infof("readLogFile: log file is empty")
	}
	return content
}

// parseProgress extracts the highest percentage from log content
// vmkfstools uses \r (carriage return) to overwrite progress lines in place
func (ct *CloneTracker) parseProgress(logContent string) float64 {
	if logContent == "" {
		return 0
	}

	var maxPct float64 = 0

	// Replace \r with \n to handle vmkfstools progress output
	// vmkfstools writes: "Clone: 0% done.\rClone: 1% done.\r..."
	normalized := strings.ReplaceAll(logContent, "\r", "\n")
	lines := strings.Split(normalized, "\n")

	klog.Infof("parseProgress: splitting into %d lines", len(lines))

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var pct float64
		// Try with period at end
		if _, err := fmt.Sscanf(line, "Clone: %f%% done.", &pct); err == nil {
			if pct > maxPct {
				maxPct = pct
				klog.Infof("parseProgress: line %d matched 'Clone: %.0f%% done.'", i, pct)
			}
			continue
		}
		// Try without period
		if _, err := fmt.Sscanf(line, "Clone: %f%% done", &pct); err == nil {
			if pct > maxPct {
				maxPct = pct
				klog.Infof("parseProgress: line %d matched 'Clone: %.0f%% done'", i, pct)
			}
		}
	}

	klog.Infof("parseProgress: final maxPct=%.0f%%", maxPct)
	return maxPct
}

// parseError checks log content for error messages
func (ct *CloneTracker) parseError(logContent string) string {
	if logContent == "" {
		return ""
	}
	// Check for failure indicators
	lowerLog := strings.ToLower(logContent)
	if strings.Contains(lowerLog, "failed") || strings.Contains(lowerLog, "error") {
		return fmt.Sprintf("vmkfstools failed: %s", logContent)
	}
	return ""
}

// determineIfRunning determines if the clone is still running based on all available signals
func (ct *CloneTracker) determineIfRunning(logContent string, percentDone float64, errorMsg string) bool {
	elapsed := time.Since(ct.startTime)

	// Check if process is visible
	processVisible, _ := ct.client.CheckCloneStatus(ct.task.Pid)
	klog.Infof("determineIfRunning: processVisible=%v, percentDone=%.0f%%, hasError=%v, logLen=%d, elapsed=%v",
		processVisible, percentDone, errorMsg != "", len(logContent), elapsed.Round(time.Second))

	// If there's an error, not running
	if errorMsg != "" {
		klog.Infof("determineIfRunning: returning false (error detected)")
		return false
	}

	// If 100% done, not running (completed)
	if percentDone >= 100 {
		klog.Infof("determineIfRunning: returning false (100%% complete)")
		return false
	}

	// If process is visible, it's running
	if processVisible {
		klog.Infof("determineIfRunning: returning true (process visible)")
		return true
	}

	// Process not visible - check if it's still working based on log
	// If log shows progress (but not 100%), clone is running even if process not visible
	if percentDone > 0 {
		klog.Infof("determineIfRunning: returning true (progress %.0f%% detected, process may be offloaded to storage)", percentDone)
		return true
	}

	// If log is empty and within startup timeout, assume still starting
	if logContent == "" && elapsed < ct.startupTimeout {
		klog.Infof("determineIfRunning: returning true (log empty, within startup timeout %v)", ct.startupTimeout)
		return true
	}

	// Process not visible, no progress, startup timeout exceeded
	klog.Infof("determineIfRunning: returning false (process not visible, no progress after %v)", elapsed.Round(time.Second))
	return false
}

// logProgressIfNeeded logs progress at 5% increments
func (ct *CloneTracker) logProgressIfNeeded(percentDone float64) {
	currentBucket := (int(percentDone) / 5) * 5
	if currentBucket > ct.lastLoggedPercent {
		elapsed := time.Since(ct.startTime).Round(time.Second)
		klog.Infof("Clone progress: %d%% done [elapsed: %v]", currentBucket, elapsed)
		ct.lastLoggedPercent = currentBucket
	}
}

// WaitForCompletion blocks until the clone completes, fails, or context is cancelled
func (ct *CloneTracker) WaitForCompletion(ctx context.Context) error {
	klog.Infof("Starting clone monitor for PID %d", ct.task.Pid)

	ticker := time.NewTicker(ct.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Info("Clone monitoring cancelled")
			return ctx.Err()

		case <-ticker.C:
			status, err := ct.GetStatus(ctx)
			if err != nil {
				return err
			}

			if !status.IsRunning {
				if status.Error != "" {
					return fmt.Errorf("clone failed: %s", status.Error)
				}
				klog.Infof("Clone completed successfully in %v", status.ElapsedTime.Round(time.Second))
				return nil
			}
		}
	}
}
