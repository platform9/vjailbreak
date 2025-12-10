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
	klog.V(2).Infof("Clone progress: %.2f%% done, before determineIfRunning", status.PercentDone)
	status.IsRunning = ct.determineIfRunning(logContent, status.PercentDone, status.Error)

	// Log progress at 5% increments
	ct.logProgressIfNeeded(status.PercentDone)

	return status, nil
}

// readLogFile reads the vmkfstools log file content
func (ct *CloneTracker) readLogFile() string {
	if ct.task.LogFile == "" {
		return ""
	}
	logCmd := fmt.Sprintf("cat %s 2>/dev/null", ct.task.LogFile)
	content, _ := ct.client.ExecuteCommand(logCmd)
	return strings.TrimSpace(content)
}

// parseProgress extracts the highest percentage from log content
func (ct *CloneTracker) parseProgress(logContent string) float64 {
	var maxPct float64 = 0
	for _, line := range strings.Split(logContent, "\n") {
		klog.V(3).Infof("Processing line: %q", line)
		var pct float64
		if _, err := fmt.Sscanf(line, "Clone: %f%% done", &pct); err == nil && pct > maxPct {
			maxPct = pct
			klog.V(3).Infof("Found new max percentage: %.2f%%", maxPct)
		}
	}
	klog.V(3).Infof("Final max percentage: %.2f%%", maxPct)
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
	// If there's an error, not running
	if errorMsg != "" {
		return false
	}

	// If 100% done, not running (completed)
	if percentDone >= 100 {
		return false
	}

	// Check if process is visible
	processVisible, _ := ct.client.CheckCloneStatus(ct.task.Pid)
	if processVisible {
		return true
	}

	// Process not visible - check if it's still working based on log
	elapsed := time.Since(ct.startTime)

	// If log shows progress (but not 100%), clone is running even if process not visible
	if percentDone > 0 {
		klog.V(2).Infof("Process not visible but log shows %.0f%% progress, treating as running", percentDone)
		return true
	}

	// If log is empty and within startup timeout, assume still starting
	if logContent == "" && elapsed < ct.startupTimeout {
		klog.V(2).Infof("Process not visible, log empty, but only %v elapsed (timeout: %v), treating as starting",
			elapsed.Round(time.Second), ct.startupTimeout)
		return true
	}

	// Process not visible, no progress, startup timeout exceeded
	klog.Infof("Clone appears to have ended: process not visible, no progress in log after %v", elapsed.Round(time.Second))
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
	klog.V(2).Infof("Clone progress: %.2f%% done just logging to check.", percentDone)
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
