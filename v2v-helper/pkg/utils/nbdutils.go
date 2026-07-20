package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/platform9/vjailbreak/pkg/common/constants"
)

// logsBaseDir is overridable so tests can point it at a temp directory.
var logsBaseDir = constants.LogsDir

func ParseFraction(text string) (int, int, error) {
	var numerator, denominator int
	_, err := fmt.Sscanf(text, "%d/%d", &numerator, &denominator)
	if err != nil {
		return 0, 0, err
	}
	return numerator, denominator, nil
}

// Log categories: each category gets its own log file per migration.
const (
	LogCategoryNBD      = "nbd"
	LogCategoryVirtV2V  = "virtv2v"
	LogCategoryGeneral  = "general"
	redactedPlaceholder = "[REDACTED]"
)

// redactingWriter strips configured substrings out of everything written
// through it before forwarding to the underlying writer. Input is buffered
// line-by-line so a match split across two Write() calls is still caught.
type redactingWriter struct {
	mu      sync.Mutex
	w       io.Writer
	secrets []string
	buf     bytes.Buffer
}

func newRedactingWriter(w io.Writer, secrets []string) *redactingWriter {
	filtered := make([]string, 0, len(secrets))
	for _, s := range secrets {
		if s != "" {
			filtered = append(filtered, s)
		}
	}
	return &redactingWriter{w: w, secrets: filtered}
}

func (r *redactingWriter) redact(s string) string {
	for _, secret := range r.secrets {
		s = strings.ReplaceAll(s, secret, redactedPlaceholder)
	}
	return s
}

func (r *redactingWriter) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	n := len(p)

	if len(r.secrets) == 0 {
		if _, err := r.w.Write(p); err != nil {
			return 0, err
		}
		return n, nil
	}

	r.buf.Write(p)
	for {
		data := r.buf.Bytes()
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			break
		}
		line := r.redact(string(data[:idx+1]))
		if _, err := r.w.Write([]byte(line)); err != nil {
			return n, err
		}
		r.buf.Next(idx + 1)
	}
	return n, nil
}

// Flush writes out any buffered partial line. Call once no more writes are
// expected, e.g. when the log file entry is being closed.
func (r *redactingWriter) Flush() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.buf.Len() == 0 {
		return nil
	}
	remaining := r.redact(r.buf.String())
	r.buf.Reset()
	_, err := r.w.Write([]byte(remaining))
	return err
}

// migrationLogKey identifies a single log file: one per (migration, category).
type migrationLogKey struct {
	migrationName string
	category      string
}

type migrationLogInfo struct {
	filePath string
	file     *os.File
	refCount int
}

type commandLogEntry struct {
	key    migrationLogKey
	writer *redactingWriter
}

var migrationLogs = make(map[migrationLogKey]*migrationLogInfo)
var commandLogs = make(map[*exec.Cmd]*commandLogEntry)
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

	entry, exists := commandLogs[cmd]
	if !exists {
		return
	}
	delete(commandLogs, cmd)

	if entry.writer != nil {
		entry.writer.Flush()
	}

	closeLogInfoLocked(entry.key)
}

// CleanupMigrationLogs forcibly closes all of a migration's log files
// regardless of reference count. Call when a migration is complete.
func CleanupMigrationLogs(migrationName string) {
	logMutex.Lock()
	defer logMutex.Unlock()

	for cmd, entry := range commandLogs {
		if entry.key.migrationName != migrationName {
			continue
		}
		if entry.writer != nil {
			entry.writer.Flush()
		}
		delete(commandLogs, cmd)
	}

	for key, logInfo := range migrationLogs {
		if key.migrationName != migrationName {
			continue
		}
		PrintLog(fmt.Sprintf("Force closing migration log file: %s", logInfo.filePath))
		logInfo.file.Sync()
		logInfo.file.Close()
		delete(migrationLogs, key)
	}
}

// RunCommandWithLogFile runs a command with output redirected to a log file
// and ensures the log file is properly closed after execution.
// WARNING: This logs the full command including all arguments. Use
// RunCommandWithLogFileRedacted instead and pass a redacted command string.
func RunCommandWithLogFile(cmd *exec.Cmd) error {
	return RunCommandWithLogFileRedactedCategory(cmd, cmd.String(), LogCategoryGeneral)
}

func RunCommandWithLogFileRedacted(cmd *exec.Cmd, cmdString string) error {
	return RunCommandWithLogFileRedactedCategory(cmd, cmdString, LogCategoryGeneral)
}

// WARNING: This logs the full command including all arguments. Use
// RunCommandWithLogFileRedactedCategory instead and pass a redacted command string.
func RunCommandWithLogFileCategory(cmd *exec.Cmd, category string) error {
	return RunCommandWithLogFileRedactedCategory(cmd, cmd.String(), category)
}

func RunCommandWithLogFileRedactedCategory(cmd *exec.Cmd, cmdString string, category string, secrets ...string) error {
	AddDebugOutputToFileWithCommandCategory(cmd, cmdString, category, secrets...)
	err := cmd.Run()
	CloseLogFile(cmd)
	return err
}

// WARNING: This logs the full command including all arguments. Use
// AddDebugOutputToFileWithCommand instead and pass a redacted command string.
func AddDebugOutputToFile(cmd *exec.Cmd) {
	AddDebugOutputToFileWithCommandCategory(cmd, cmd.String(), LogCategoryGeneral)
}

func AddDebugOutputToFileWithCommand(cmd *exec.Cmd, cmdString string) {
	AddDebugOutputToFileWithCommandCategory(cmd, cmdString, LogCategoryGeneral)
}

// WARNING: This logs the full command including all arguments. Use
// AddDebugOutputToFileWithCommandCategory instead and pass a redacted command string.
func AddDebugOutputToFileCategory(cmd *exec.Cmd, category string) {
	AddDebugOutputToFileWithCommandCategory(cmd, cmd.String(), category)
}

// AddDebugOutputToFileWithCommandCategory redirects command output to a
// per-migration, per-category debug log file. Any secrets passed in are
// scrubbed from the command's stdout/stderr before it's written.
func AddDebugOutputToFileWithCommandCategory(cmd *exec.Cmd, cmdString string, category string, secrets ...string) {
	if category == "" {
		category = LogCategoryGeneral
	}

	migrationName, err := GetMigrationObjectName()
	if err != nil {
		return
	}

	if err := os.MkdirAll(logsBaseDir, 0755); err != nil {
		return
	}

	attemptDir := fmt.Sprintf("%s/%s", logsBaseDir, migrationName)
	if err := os.MkdirAll(attemptDir, 0755); err != nil {
		return
	}

	logMutex.Lock()
	defer logMutex.Unlock()

	key := migrationLogKey{migrationName: migrationName, category: category}

	if oldEntry, exists := commandLogs[cmd]; exists {
		delete(commandLogs, cmd)
		if oldEntry.writer != nil {
			oldEntry.writer.Flush()
		}
		closeLogInfoLocked(oldEntry.key)
	}

	var logFile *os.File
	logInfo, exists := migrationLogs[key]
	if exists {
		logFile = logInfo.file
		logInfo.refCount++
	} else {
		timestamp := time.Now().Format("2006-01-02-15:04:05")
		logFilePath := fmt.Sprintf("%s/%s.%s.log", attemptDir, category, timestamp)

		var err error
		logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return
		}

		migrationLogs[key] = &migrationLogInfo{
			filePath: logFilePath,
			file:     logFile,
			refCount: 1,
		}

		headerTime := time.Now().Format("2006-01-02 15:04:05")
		logFile.WriteString(fmt.Sprintf("==== MIGRATION LOG [%s]: %s - Started at %s ====\n\n",
			category, migrationName, headerTime))
	}

	writer := newRedactingWriter(logFile, secrets)
	commandLogs[cmd] = &commandLogEntry{key: key, writer: writer}

	timePrefix := time.Now().Format("2006-01-02 15:04:05")
	logFile.WriteString(fmt.Sprintf("\n\n==== %s: COMMAND: %s ====\n\n", timePrefix, cmdString))

	cmd.Stdout = writer
	cmd.Stderr = writer
	// NOTE: The log file is not automatically closed or flushed when the
	// command completes. Call CloseLogFile(cmd), or use RunCommandWithLogFile
	// (or its *Category variant) which handles cleanup automatically.
}
