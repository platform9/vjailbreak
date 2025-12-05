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
	client       *Client
	task         *VmkfstoolsTask
	sourcePath   string
	targetPath   string
	startTime    time.Time
	lastChecksum int64
	pollInterval time.Duration
}

// NewCloneTracker creates a new clone operation tracker
func NewCloneTracker(client *Client, task *VmkfstoolsTask, sourcePath, targetPath string) *CloneTracker {
	return &CloneTracker{
		client:       client,
		task:         task,
		sourcePath:   sourcePath,
		targetPath:   targetPath,
		startTime:    time.Now(),
		pollInterval: 2 * time.Second,
	}
}

// SetPollInterval sets how often to check clone status
func (ct *CloneTracker) SetPollInterval(interval time.Duration) {
	ct.pollInterval = interval
}

// GetStatus returns the current status of the clone operation
func (ct *CloneTracker) GetStatus() (*CloneStatus, error) {
	status := &CloneStatus{
		PID:         ct.task.Pid,
		ElapsedTime: time.Since(ct.startTime),
	}

	// Check if process is still running
	isRunning, err := ct.client.CheckCloneStatus(ct.task.Pid)
	if err != nil {
		return nil, fmt.Errorf("failed to check clone status: %w", err)
	}
	status.IsRunning = isRunning

	// Always check the log file for progress and errors
	logContent := ""
	if ct.task.LogFile != "" {
		logCmd := fmt.Sprintf("cat %s 2>/dev/null", ct.task.LogFile)
		logContent, _ = ct.client.ExecuteCommand(logCmd)
		logContent = strings.TrimSpace(logContent)
		klog.V(2).Infof("Log file content: %s", logContent)
	}

	// Parse progress from log (vmkfstools outputs "Clone: XX% done.")
	if strings.Contains(logContent, "% done") {
		// Extract percentage from log
		for _, line := range strings.Split(logContent, "\n") {
			if strings.Contains(line, "% done") {
				var pct float64
				if _, err := fmt.Sscanf(line, "Clone: %f%% done", &pct); err == nil {
					status.PercentDone = pct
					klog.V(2).Infof("Parsed progress: %.1f%%", pct)
				}
			}
		}
	}

	// If not running, check log for success/failure
	if !isRunning {
		klog.Infof("Process %d is no longer running, checking log for result", ct.task.Pid)
		klog.Infof("Log content: %s", logContent)

		// Check for errors in log
		if strings.Contains(logContent, "Failed") || strings.Contains(logContent, "Error") || strings.Contains(logContent, "error") {
			status.Error = fmt.Sprintf("vmkfstools failed: %s", logContent)
			return status, nil
		}

		// Check for successful completion (100% done in log)
		if strings.Contains(logContent, "100% done") {
			klog.Infof("Clone completed successfully (100%% done in log)")
			status.PercentDone = 100.0
			return status, nil
		}

		// Also check if RDM descriptor was created (indicates success)
		if ct.task.RDMDescriptorPath != "" {
			checkCmd := fmt.Sprintf("test -f %s && echo 'exists'", ct.task.RDMDescriptorPath)
			output, _ := ct.client.ExecuteCommand(checkCmd)
			if strings.Contains(output, "exists") {
				klog.Infof("RDM descriptor exists at %s, clone successful", ct.task.RDMDescriptorPath)
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
