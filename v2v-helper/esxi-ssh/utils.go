// Copyright © 2024 The vjailbreak authors

package esxissh

import (
	"fmt"
	"time"
)

// RetryConfig configures retry behavior for operations
type RetryConfig struct {
	MaxAttempts int
	InitialDelay time.Duration
	MaxDelay time.Duration
	Multiplier float64
}

// DefaultRetryConfig returns sensible retry defaults
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts: 3,
		InitialDelay: 1 * time.Second,
		MaxDelay: 30 * time.Second,
		Multiplier: 2.0,
	}
}

// RetryWithBackoff executes a function with exponential backoff
func RetryWithBackoff(config *RetryConfig, operation func() error) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't sleep on the last attempt
		if attempt < config.MaxAttempts {
			time.Sleep(delay)

			// Calculate next delay with exponential backoff
			delay = time.Duration(float64(delay) * config.Multiplier)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// ConnectWithRetry connects to ESXi with retry logic
func (c *Client) ConnectWithRetry(config *RetryConfig) error {
	return RetryWithBackoff(config, func() error {
		return c.Connect()
	})
}

// FormatBytes formats bytes into human-readable string
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.2f %s", float64(bytes)/float64(div), units[exp])
}

// FormatDuration formats duration into human-readable string
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hours, minutes)
}

// FormatTransferSpeed formats bytes/second into human-readable string
func FormatTransferSpeed(bytesPerSecond float64) string {
	return fmt.Sprintf("%s/s", FormatBytes(int64(bytesPerSecond)))
}
