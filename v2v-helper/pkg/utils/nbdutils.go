package utils

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/platform9/vjailbreak/v2v-helper/pkg/constants"
)

func ParseFraction(text string) (int, int, error) {
	var numerator, denominator int
	_, err := fmt.Sscanf(text, "%d/%d", &numerator, &denominator)
	if err != nil {
		return 0, 0, err
	}
	return numerator, denominator, nil
}

func AddDebugOutputToFile(cmd *exec.Cmd) {
	migrationName, err := GetMigrationObjectName()
	if err != nil {
		return
	}
	
	// Ensure logs directory exists
	if err := os.MkdirAll(constants.LogsDir, 0755); err != nil {
		return
	}
	
	// Create a timestamped log filename to differentiate between migration attempts
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	attemptDir := fmt.Sprintf("%s/%s", constants.LogsDir, migrationName)
	
	// Create a subdirectory for the specific migration if it doesn't exist
	if err := os.MkdirAll(attemptDir, 0755); err != nil {
		return
	}
	
	// Check if any attempts already exist
	files, err := os.ReadDir(attemptDir)
	
	attemptNum := 1
	if err == nil {
		// Count existing attempt directories
		attemptNum = len(files) + 1
	}
	
	// Create an attempt-specific log file
	logFilePath := fmt.Sprintf("%s/attempt-%d-%s.log", attemptDir, attemptNum, timestamp)
	
	// Create new file or append if it already exists (same attempt)
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	
	PrintLog(fmt.Sprintf("Debug mode enabled. Command output will be logged to %s", logFilePath))

	// Set stdout/stderr to only write to the log file, not to kubectl logs
	cmd.Stdout = logFile
	cmd.Stderr = logFile
}
