// Copyright © 2026 The vjailbreak authors

// Package timing records how long each vCenter, PCD and disk-copy operation
// takes during a migration and emits the result as machine-parseable log lines.
//
// It exists so two runs of the same VM — one with the Hot-Add proxy and one
// without — can be diffed call-by-call instead of eyeballing log timestamps.
// Every step emits an immediate line:
//
//	[TIMING] step="PCD: Create Cinder Volume" duration_ms=1043 err=false
//
// and at the end of the migration a single aggregated line:
//
//	[TIMING-SUMMARY] {"vm":"win2k19","method":"HotAdd",...}
//
// A nil *Recorder is safe to use — every method is a no-op — so callers do not
// need to branch on whether timing is enabled.
package timing

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// LinePrefix marks a single timed step. Kept as a constant so the report
// script and the tests agree on exactly one spelling.
const LinePrefix = "[TIMING]"

// SummaryPrefix marks the one-line JSON aggregate emitted at end of migration.
const SummaryPrefix = "[TIMING-SUMMARY]"

// Stat is the aggregate for one step name across a whole migration.
type Stat struct {
	// Step is the human-readable operation name, e.g. "vCenter: Take Snapshot".
	Step string `json:"step"`
	// Count is how many times the step ran. Steps that run per-disk or per-NIC
	// have Count > 1, which is why TotalMS alone is not enough to compare runs.
	Count int64 `json:"count"`
	// TotalMS is the summed wall-clock duration of every invocation.
	TotalMS int64 `json:"total_ms"`
	// MaxMS is the slowest single invocation.
	MaxMS int64 `json:"max_ms"`
	// Errors counts invocations that returned a non-nil error. A step with
	// errors > 0 spent part of its time on retries and is not comparable
	// across runs without saying so.
	Errors int64 `json:"errors"`

	// order is the sequence in which the step was first seen. Not serialised;
	// used only to keep summary output in call order rather than map order.
	order int
}

// Summary is the payload of the [TIMING-SUMMARY] line.
type Summary struct {
	VM string `json:"vm"`
	// Method is the StorageCopyMethod in effect: "HotAdd", "StorageAcceleratedCopy"
	// or "NBD" for the default nbdkit/VDDK path.
	Method string `json:"method"`
	// MigrationType is "hot" or "cold". A HotAdd run is only comparable against
	// a cold NBD run — a hot run adds changed-block iterations that HotAdd has no
	// equivalent for.
	MigrationType string `json:"migration_type"`
	// DiskCount, DiskBytes and AllocatedBytes normalise the comparison. HotAdd
	// reads a raw block device and moves every byte; the VDDK path skips
	// unallocated extents. Without AllocatedBytes the two runs cannot be
	// compared honestly on a thin-provisioned disk.
	DiskCount      int    `json:"disk_count"`
	DiskBytes      int64  `json:"disk_bytes"`
	AllocatedBytes int64  `json:"allocated_bytes"`
	TotalMS        int64  `json:"total_ms"`
	Failed         bool   `json:"failed"`
	Steps          []Stat `json:"steps"`
}

// Recorder accumulates per-step timings for one migration. Safe for concurrent
// use: the Hot-Add path copies disks from parallel goroutines.
type Recorder struct {
	mu    sync.Mutex
	steps map[string]*Stat
	next  int

	start          time.Time
	vmName         string
	method         string
	migrationType  string
	diskCount      int
	diskBytes      int64
	allocatedBytes int64

	// emit writes one line. It is injected rather than calling utils.PrintLog
	// directly because pkg/utils imports the vm package, and the vm package
	// imports this one — depending on utils here would close an import cycle.
	emit func(string)
}

// New returns a Recorder that writes each line through emit. Pass
// utils.PrintLog from a package that is allowed to import it (main, migrate).
// A nil emit is replaced with a no-op.
func New(vmName, method, migrationType string, emit func(string)) *Recorder {
	if emit == nil {
		emit = func(string) {}
	}
	return newWithEmitter(vmName, method, migrationType, emit)
}

func newWithEmitter(vmName, method, migrationType string, emit func(string)) *Recorder {
	if method == "" {
		method = "NBD"
	}
	return &Recorder{
		steps:         map[string]*Stat{},
		start:         time.Now(),
		vmName:        vmName,
		method:        method,
		migrationType: migrationType,
		emit:          emit,
	}
}

// SetDiskInfo records the source disk footprint used to normalise the two runs.
// allocatedBytes may be 0 when the source does not report allocation.
func (r *Recorder) SetDiskInfo(count int, totalBytes, allocatedBytes int64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.diskCount = count
	r.diskBytes = totalBytes
	r.allocatedBytes = allocatedBytes
}

// Start begins timing step and returns the function that ends it. Pass the
// operation's error to the returned function so failed calls are excluded from
// clean comparisons:
//
//	done := rec.Start("PCD: Create Cinder Volume")
//	vol, err := client.CreateVolume(ctx, ...)
//	done(err)
//
// Start is the general form because the wrapped calls return many different
// types. Use Track for plain error-returning calls.
func (r *Recorder) Start(step string) func(error) {
	if r == nil {
		return func(error) {}
	}
	begin := time.Now()
	return func(err error) {
		r.record(step, time.Since(begin), err)
	}
}

// Track times an error-returning call.
func (r *Recorder) Track(step string, fn func() error) error {
	if r == nil {
		return fn()
	}
	done := r.Start(step)
	err := fn()
	done(err)
	return err
}

func (r *Recorder) record(step string, d time.Duration, err error) {
	ms := d.Milliseconds()

	r.mu.Lock()
	stat, ok := r.steps[step]
	if !ok {
		stat = &Stat{Step: step, order: r.next}
		r.next++
		r.steps[step] = stat
	}
	stat.Count++
	stat.TotalMS += ms
	if ms > stat.MaxMS {
		stat.MaxMS = ms
	}
	if err != nil {
		stat.Errors++
	}
	emit := r.emit
	r.mu.Unlock()

	if emit != nil {
		emit(fmt.Sprintf("%s step=%q duration_ms=%d err=%t", LinePrefix, step, ms, err != nil))
	}
}

// Snapshot returns the accumulated stats in first-seen order.
func (r *Recorder) Snapshot() []Stat {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked()
}

func (r *Recorder) snapshotLocked() []Stat {
	out := make([]Stat, 0, len(r.steps))
	for _, s := range r.steps {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].order < out[j].order })
	return out
}

// Summarise builds the aggregate without emitting it. Exported for tests and
// for callers that want the numbers in-process.
func (r *Recorder) Summarise(failed bool) Summary {
	if r == nil {
		return Summary{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return Summary{
		VM:             r.vmName,
		Method:         r.method,
		MigrationType:  r.migrationType,
		DiskCount:      r.diskCount,
		DiskBytes:      r.diskBytes,
		AllocatedBytes: r.allocatedBytes,
		TotalMS:        time.Since(r.start).Milliseconds(),
		Failed:         failed,
		Steps:          r.snapshotLocked(),
	}
}

// EmitSummary writes the single [TIMING-SUMMARY] JSON line. Call it on both the
// success and the failure path — a failed run's partial timings are still the
// fastest way to see which step ate the time.
func (r *Recorder) EmitSummary(failed bool) {
	if r == nil {
		return
	}
	summary := r.Summarise(failed)
	payload, err := json.Marshal(summary)
	if err != nil {
		// Never fail a migration over a metrics line.
		r.emit(fmt.Sprintf("%s marshal failed: %v", SummaryPrefix, err))
		return
	}
	r.emit(fmt.Sprintf("%s %s", SummaryPrefix, string(payload)))
}
