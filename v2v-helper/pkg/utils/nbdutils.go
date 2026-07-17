package utils

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/platform9/vjailbreak/pkg/common/constants"
)

// logsBaseDir is the root directory under which per-migration/per-category log
// files are created. It defaults to constants.LogsDir but is a variable
// (rather than using the constant directly) so unit tests can point it at a
// temp directory instead of the real, often-unwritable, /var/log/pf9 path.
var logsBaseDir = constants.LogsDir

func ParseFraction(text string) (int, int, error) {
	var numerator, denominator int
	_, err := fmt.Sscanf(text, "%d/%d", &numerator, &denominator)
	if err != nil {
		return 0, 0, err
	}
	return numerator, denominator, nil
}

// Log categories: each category gets its own log file per migration so that
// noisy subprocess output (e.g. nbdkit/nbdcopy) doesn't drown out virt-v2v
// output (or vice versa) in a single interleaved file.
const (
	// LogCategoryNBD groups nbdkit/nbdcopy disk-copy related command output.
	LogCategoryNBD = "nbd"
	// LogCategoryVirtV2V groups virt-v2v-in-place and related guest
	// conversion/customization command output (e.g. ntfsfix).
	LogCategoryVirtV2V = "virtv2v"
	// LogCategoryGeneral is the default bucket for commands that don't
	// belong to a more specific category.
	LogCategoryGeneral = "general"
)

// migrationLogKey identifies a single log file: one per (migration, category) pair.
type migrationLogKey struct {
	migrationName string
	category      string
}

// migrationLogInfo tracks log files by migration+category to ensure one file
// per migration attempt per category.
type migrationLogInfo struct {
	filePath string
	file     *os.File
	refCount int
}

// We use a map to track migration log files, keyed by migration name + category.
var migrationLogs = make(map[migrationLogKey]*migrationLogInfo)

// commandLogs maps commands to their migration log key for cleanup
var commandLogs = make(map[*exec.Cmd]migrationLogKey)
var logMutex sync.Mutex

// closeLogInfoLocked decrements the refcount for a log key and closes/removes
// the underlying file if no longer referenced. Caller must hold logMutex.
func closeLogInfoLocked(key migrationLogKey) {
	logInfo, ok := migrationLogs[key]
	if !ok {
		return
	}
	logInfo.refCount--
	if logInfo.refCount <= 0 {
		// Last command using this log file, close it
		PrintLog(fmt.Sprintf("Closing migration log file: %s", logInfo.filePath))
		logInfo.file.Sync()
		logInfo.file.Close()
		delete(migrationLogs, key)
	}
}

// CloseLogFile decreases reference count for a migration log file and closes it if no longer needed
func CloseLogFile(cmd *exec.Cmd) {
	logMutex.Lock()
	defer logMutex.Unlock()

	// Get the log key associated with this command
	key, exists := commandLogs[cmd]
	if !exists {
		return // No log file associated with this command
	}

	// Remove command tracking
	delete(commandLogs, cmd)

	closeLogInfoLocked(key)
}

// CleanupMigrationLogs forcibly closes all of a migration's log files (across
// all categories) regardless of reference count.
// Call this when a migration is complete to clean up resources
func CleanupMigrationLogs(migrationName string) {
	logMutex.Lock()
	defer logMutex.Unlock()

	// Close and remove any open log files for this migration, across all categories
	for key, logInfo := range migrationLogs {
		if key.migrationName != migrationName {
			continue
		}
		PrintLog(fmt.Sprintf("Force closing migration log file: %s", logInfo.filePath))
		logInfo.file.Sync()
		logInfo.file.Close()
		delete(migrationLogs, key)
	}

	// Clean up any commands associated with this migration
	for cmd, key := range commandLogs {
		if key.migrationName == migrationName {
			delete(commandLogs, cmd)
		}
	}
}

// RunCommandWithLogFile runs a command with output redirected to a log file
// and ensures the log file is properly closed after execution
// WARNING: This logs the full command including all arguments. Do NOT use this
// for commands containing sensitive data (passwords, tokens, etc.). Use
// RunCommandWithLogFileRedacted instead and pass a redacted command string.
func RunCommandWithLogFile(cmd *exec.Cmd) error {
	return RunCommandWithLogFileRedactedCategory(cmd, cmd.String(), LogCategoryGeneral)
}

// RunCommandWithLogFileRedacted runs a command with output redirected to the
// default ("general") log file using a redacted command string for logging,
// and ensures the log file is properly closed.
func RunCommandWithLogFileRedacted(cmd *exec.Cmd, cmdString string) error {
	return RunCommandWithLogFileRedactedCategory(cmd, cmdString, LogCategoryGeneral)
}

// RunCommandWithLogFileCategory runs a command with output redirected to the
// log file for the given category, and ensures the log file is properly closed.
// WARNING: This logs the full command including all arguments. Do NOT use this
// for commands containing sensitive data. Use RunCommandWithLogFileRedactedCategory
// instead and pass a redacted command string.
func RunCommandWithLogFileCategory(cmd *exec.Cmd, category string) error {
	return RunCommandWithLogFileRedactedCategory(cmd, cmd.String(), category)
}

// RunCommandWithLogFileRedactedCategory runs a command with output redirected
// to the log file for the given category, using a redacted command string for
// logging, and ensures the log file is properly closed after execution.
func RunCommandWithLogFileRedactedCategory(cmd *exec.Cmd, cmdString string, category string) error {
	// First ensure output is directed to a log file with redacted command
	AddDebugOutputToFileWithCommandCategory(cmd, cmdString, category)

	// Run the command
	err := cmd.Run()

	// Close the log file
	CloseLogFile(cmd)

	return err
}

// AddDebugOutputToFile redirects command output to the default ("general")
// debug log file.
// WARNING: This logs the full command including all arguments. Do NOT use this
// for commands containing sensitive data (passwords, tokens, etc.). Use
// AddDebugOutputToFileWithCommand instead and pass a redacted command string.
func AddDebugOutputToFile(cmd *exec.Cmd) {
	AddDebugOutputToFileWithCommandCategory(cmd, cmd.String(), LogCategoryGeneral)
}

// AddDebugOutputToFileWithCommand redirects command output to the default
// ("general") debug log file, using the provided (already-redacted) command string.
func AddDebugOutputToFileWithCommand(cmd *exec.Cmd, cmdString string) {
	AddDebugOutputToFileWithCommandCategory(cmd, cmdString, LogCategoryGeneral)
}

// AddDebugOutputToFileCategory redirects command output to the debug log file
// for the given category.
// WARNING: This logs the full command including all arguments. Do NOT use this
// for commands containing sensitive data. Use
// AddDebugOutputToFileWithCommandCategory instead and pass a redacted command string.
func AddDebugOutputToFileCategory(cmd *exec.Cmd, category string) {
	AddDebugOutputToFileWithCommandCategory(cmd, cmd.String(), category)
}

// AddDebugOutputToFileWithCommandCategory redirects command output to a
// per-migration, per-category debug log file (e.g. "nbd.<timestamp>.log" or
// "virtv2v.<timestamp>.log"), keeping different tools' noisy debug output in
// separate files so they're easy to find independently.
func AddDebugOutputToFileWithCommandCategory(cmd *exec.Cmd, cmdString string, category string) {
	if category == "" {
		category = LogCategoryGeneral
	}

	migrationName, err := GetMigrationObjectName()
	if err != nil {
		return
	}

	// Ensure logs directory exists
	if err := os.MkdirAll(logsBaseDir, 0755); err != nil {
		return
	}

	// Create a directory for this specific migration
	attemptDir := fmt.Sprintf("%s/%s", logsBaseDir, migrationName)
	if err := os.MkdirAll(attemptDir, 0755); err != nil {
		return
	}

	logMutex.Lock()
	defer logMutex.Unlock()

	key := migrationLogKey{migrationName: migrationName, category: category}

	// First close any existing log file for this command
	// (do this within lock to avoid race conditions)
	if oldKey, exists := commandLogs[cmd]; exists {
		// Remove command tracking
		delete(commandLogs, cmd)
		closeLogInfoLocked(oldKey)
	}

	// Check if we already have an open log file for this migration+category
	var logFile *os.File
	logInfo, exists := migrationLogs[key]
	if exists {
		// Reuse existing log file
		logFile = logInfo.file
		logInfo.refCount++ // Increment reference count
	} else {
		// Create a new log file with timestamp in name (only done once per migration+category)
		timestamp := time.Now().Format("2006-01-02-15:04:05")
		logFilePath := fmt.Sprintf("%s/%s.%s.log", attemptDir, category, timestamp)

		// Create/open the log file
		var err error
		logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return
		}

		// Store new log info
		migrationLogs[key] = &migrationLogInfo{
			filePath: logFilePath,
			file:     logFile,
			refCount: 1,
		}

		// Write header to log file
		headerTime := time.Now().Format("2006-01-02 15:04:05")
		logFile.WriteString(fmt.Sprintf("==== MIGRATION LOG [%s]: %s - Started at %s ====\n\n",
			category, migrationName, headerTime))
	}

	// Associate this command with the migration+category for later tracking
	commandLogs[cmd] = key

	// Write a separator for this command to make logs more readable
	timePrefix := time.Now().Format("2006-01-02 15:04:05")
	logFile.WriteString(fmt.Sprintf("\n\n==== %s: COMMAND: %s ====\n\n", timePrefix, cmdString))

	// Redirect command output to this file
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	// NOTE: The log file will NOT be automatically closed when the command completes.
	// Either call CloseLogFile(cmd) manually after the command completes,
	// or use RunCommandWithLogFile() (or its *Category variant) which handles
	// the cleanup automatically.
}
