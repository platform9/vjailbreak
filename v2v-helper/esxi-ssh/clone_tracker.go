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

	// If not running, verify completion
	if !isRunning {
		// Check if target files exist
		exists, _ := ct.client.CheckVMDKExists(ct.targetPath)
		if exists {
			// Clone completed successfully
			status.PercentDone = 100.0
			status.BytesCopied = status.TotalBytes
		} else {
			// Clone may have failed
			status.Error = "Clone process ended but target files not found"
		}
		return status, nil
	}

	// Get target directory size to estimate progress
	targetDir := ct.targetPath[:strings.LastIndex(ct.targetPath, "/")]
	sizeCmd := fmt.Sprintf("du -sb %s 2>/dev/null | awk '{print $1}'", targetDir)
	sizeOutput, err := ct.client.ExecuteCommand(sizeCmd)
	if err == nil && sizeOutput != "" {
		var bytesCopied int64
		if _, err := fmt.Sscanf(strings.TrimSpace(sizeOutput), "%d", &bytesCopied); err == nil {
			status.BytesCopied = bytesCopied

			// Get total size from source
			if sourceSize, err := ct.client.GetVMDKSize(ct.sourcePath); err == nil {
				status.TotalBytes = sourceSize
				if sourceSize > 0 {
					status.PercentDone = float64(bytesCopied) / float64(sourceSize) * 100.0

					// Estimate time remaining
					if status.PercentDone > 0 && status.PercentDone < 100 {
						totalEstimated := time.Duration(float64(status.ElapsedTime) / (status.PercentDone / 100.0))
						status.EstimatedTime = totalEstimated - status.ElapsedTime
					}
				}
			}
		}
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
