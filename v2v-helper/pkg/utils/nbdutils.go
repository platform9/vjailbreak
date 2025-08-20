package utils

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
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

// openLogFiles tracks active log files that need to be closed properly
var openLogFiles = make(map[*exec.Cmd]*os.File)
var logFileMutex sync.Mutex

// CloseLogFile closes the log file associated with a command
func CloseLogFile(cmd *exec.Cmd) {
	logFileMutex.Lock()
	defer logFileMutex.Unlock()
	
	if logFile, exists := openLogFiles[cmd]; exists {
		PrintLog(fmt.Sprintf("Closing log file: %s", logFile.Name()))
		logFile.Sync()
		logFile.Close()
		delete(openLogFiles, cmd)
	}
}

// RunCommandWithLogFile runs a command with output redirected to a log file
// and ensures the log file is properly closed after execution
func RunCommandWithLogFile(cmd *exec.Cmd) error {
	// First ensure output is directed to a log file
	AddDebugOutputToFile(cmd)
	
	// Run the command
	err := cmd.Run()
	
	// Close the log file
	CloseLogFile(cmd)
	
	return err
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
	
	// First close any existing log file for this command
	CloseLogFile(cmd)
	
	// Create new file or append if it already exists (same attempt)
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	
	// Keep track of the open file so we can close it later
	logFileMutex.Lock()
	openLogFiles[cmd] = logFile
	logFileMutex.Unlock()
	
	PrintLog(fmt.Sprintf("Debug mode enabled. Command output will be logged to %s", logFilePath))

	// Set stdout/stderr to only write to the log file, not to kubectl logs
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	
	// NOTE: The log file will NOT be automatically closed.
	// Either call CloseLogFile(cmd) manually after the command completes,
	// or use RunCommandWithLogFile() which handles the cleanup automatically.
}
