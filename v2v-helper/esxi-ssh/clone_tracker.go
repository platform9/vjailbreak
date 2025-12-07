// Copyright © 2024 The vjailbreak authors

package esxissh

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// CloneStatus represents the current state of a clone operation
type CloneStatus struct {
	PID           int
	IsRunning     bool
	BytesCopied   int64
	TotalBytes    int64
	PercentDone   float64
	ElapsedTime   time.Duration
	EstimatedTime time.Duration
	Error         string
}

// CloneTracker monitors a vmkfstools clone operation in real-time
type CloneTracker struct {
	client         *Client
	task           *VmkfstoolsTask
	sourcePath     string
	targetPath     string
	startTime      time.Time
	lastChecksum   int64
	pollInterval   time.Duration
	startupTimeout time.Duration // Time to wait for process to actually start executing
}

// NewCloneTracker creates a new clone operation tracker
func NewCloneTracker(client *Client, task *VmkfstoolsTask, sourcePath, targetPath string) *CloneTracker {
	return &CloneTracker{
		client:         client,
		task:           task,
		sourcePath:     sourcePath,
		targetPath:     targetPath,
		startTime:      time.Now(),
		pollInterval:   2 * time.Second,
		startupTimeout: 5 * time.Minute, // vmkfstools can take 30+ seconds to start executing
	}
}

// SetPollInterval sets how often to check clone status
func (ct *CloneTracker) SetPollInterval(interval time.Duration) {
	ct.pollInterval = interval
}

// SetStartupTimeout sets how long to wait for the process to actually start executing
// vmkfstools can take 30+ seconds to start after the shell returns the PID
func (ct *CloneTracker) SetStartupTimeout(timeout time.Duration) {
	ct.startupTimeout = timeout
}

// GetStatus returns the current status of the clone operation
func (ct *CloneTracker) GetStatus() (*CloneStatus, error) {
	status := &CloneStatus{
		PID:         ct.task.Pid,
		ElapsedTime: time.Since(ct.startTime),
	}

	// Always check the log file FIRST for progress and errors
	// This is more reliable than checking process status immediately after start
	logContent := ""
	if ct.task.LogFile != "" {
		logCmd := fmt.Sprintf("cat %s 2>/dev/null", ct.task.LogFile)
		logContent, _ = ct.client.ExecuteCommand(logCmd)
		logContent = strings.TrimSpace(logContent)
		klog.V(2).Infof("Log file content: %s", logContent)
	}

	// Check if process is still running
	isRunning, err := ct.client.CheckCloneStatus(ct.task.Pid)
	if err != nil {
		return nil, fmt.Errorf("failed to check clone status: %w", err)
	}
	status.IsRunning = isRunning

	// Parse progress from log (vmkfstools outputs "Clone: XX% done.")
	if strings.Contains(logContent, "% done") {
		// Extract percentage from log
		for _, line := range strings.Split(logContent, "\n") {
			if strings.Contains(line, "% done") {
				var pct float64
				if _, err := fmt.Sscanf(line, "Clone: %f%% done", &pct); err == nil {
					status.PercentDone = pct
					klog.V(2).Infof("Copying progress: %.1f%%", pct)
				}
			}
		}
	}

	// If not running according to ps, we need to determine the actual state
	// vmkfstools may not be visible via ps even while running
	if !isRunning {
		klog.Infof("Process %d is not visible via ps, checking log for actual state", ct.task.Pid)
		klog.Infof("Log content: %s", logContent)

		elapsedTime := time.Since(ct.startTime)

		// Check for errors in log FIRST - this is definitive
		if strings.Contains(logContent, "Failed") || strings.Contains(logContent, "Error") || strings.Contains(logContent, "error") {
			status.Error = fmt.Sprintf("vmkfstools failed: %s", logContent)
			return status, nil
		}

		// Check for successful completion (100% done in log) - this is definitive
		if strings.Contains(logContent, "100% done") {
			klog.Infof("Clone completed successfully (100%% done in log)")
			status.PercentDone = 100.0
			return status, nil
		}

		// If log shows progress but not 100%, the clone is STILL RUNNING
		// (vmkfstools may not be visible via ps but it's definitely working)
		if strings.Contains(logContent, "% done") && status.PercentDone < 100 {
			klog.Infof("Log shows %0.1f%% progress - clone is still running (process may not be visible via ps)", status.PercentDone)
			status.IsRunning = true
			return status, nil
		}

		// If log has "Cloning disk" but no progress yet, clone just started - still running
		if strings.Contains(logContent, "Cloning disk") {
			klog.Infof("Log shows 'Cloning disk' - clone has started and is running")
			status.IsRunning = true
			return status, nil
		}

		// If log is empty and we're within startup timeout, process may still be starting
		if logContent == "" && elapsedTime < ct.startupTimeout {
			klog.Infof("Process %d not found but only %v elapsed (startup timeout: %v), treating as still starting",
				ct.task.Pid, elapsedTime.Round(time.Second), ct.startupTimeout)
			status.IsRunning = true
			return status, nil
		}

		// At this point: no error, no 100% done, no progress, no "Cloning disk", and either:
		// - log is empty and startup timeout exceeded, OR
		// - log has unexpected content
		// Check if RDM descriptor exists as final fallback (only valid if log is empty/missing)
		if ct.task.RDMDescriptorPath != "" && logContent == "" {
			checkCmd := fmt.Sprintf("test -f %s && echo 'exists'", ct.task.RDMDescriptorPath)
			output, _ := ct.client.ExecuteCommand(checkCmd)
			if strings.Contains(output, "exists") {
				klog.Infof("RDM descriptor exists at %s and log is empty - assuming clone successful", ct.task.RDMDescriptorPath)
				status.PercentDone = 100.0
				return status, nil
			}
		}

		// Process ended but no success indicators
		if logContent == "" {
			status.Error = "Clone process ended but log file is empty - process may have failed to start"
		} else {
			status.Error = fmt.Sprintf("Clone process ended unexpectedly. Log: %s", logContent)
		}
		return status, nil
	}

	// Process is still running - get size info for progress estimation
	if sourceSize, err := ct.client.GetVMDKSize(ct.sourcePath); err == nil {
		status.TotalBytes = sourceSize
	}

	return status, nil
}

// Monitor continuously monitors the clone until completion or error
// Calls the callback function with status updates at each poll interval
func (ct *CloneTracker) Monitor(callback func(*CloneStatus) bool) error {
	klog.Infof("Starting clone monitor for PID %d", ct.task.Pid)

	for {
		status, err := ct.GetStatus()
		if err != nil {
			return fmt.Errorf("failed to get status: %w", err)
		}

		// Call callback - if it returns false, stop monitoring
		if callback != nil {
			if !callback(status) {
				klog.Info("Monitor stopped by callback")
				return nil
			}
		}

		// If clone is no longer running, we're done
		if !status.IsRunning {
			if status.Error != "" {
				return fmt.Errorf("clone failed: %s", status.Error)
			}
			klog.Info("Clone completed successfully")
			return nil
		}

		// Wait before next check
		time.Sleep(ct.pollInterval)
	}
}

// WaitForCompletion blocks until the clone completes or fails
func (ct *CloneTracker) WaitForCompletion() error {
	return ct.Monitor(nil)
}

// MonitorWithProgress monitors and prints progress updates
func (ct *CloneTracker) MonitorWithProgress() error {
	return ct.Monitor(func(status *CloneStatus) bool {
		if status.IsRunning {
			if status.TotalBytes > 0 {
				klog.Infof("[%s] Clone progress: %.1f%% (%s / %s)",
					status.ElapsedTime.Round(time.Second),
					status.PercentDone,
					formatBytes(status.BytesCopied),
					formatBytes(status.TotalBytes))
			} else {
				klog.Infof("[%s] Clone in progress...", status.ElapsedTime.Round(time.Second))
			}
		}
		return true // Continue monitoring
	})
}

// Helper function to format bytes
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
