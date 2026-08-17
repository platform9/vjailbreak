// Copyright © 2026 The vjailbreak authors

package timing

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

// capture returns a Recorder that appends emitted lines to a slice instead of
// writing to the migration log file.
func capture(t *testing.T) (*Recorder, *[]string) {
	t.Helper()
	var mu sync.Mutex
	lines := []string{}
	rec := newWithEmitter("test-vm", "HotAdd", "cold", func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, msg)
	})
	return rec, &lines
}

func TestTrackAccumulates(t *testing.T) {
	tests := []struct {
		name        string
		invocations []error
		wantCount   int64
		wantErrors  int64
	}{
		{name: "single success", invocations: []error{nil}, wantCount: 1, wantErrors: 0},
		{name: "repeated success", invocations: []error{nil, nil, nil}, wantCount: 3, wantErrors: 0},
		{name: "mixed", invocations: []error{nil, errors.New("boom"), nil}, wantCount: 3, wantErrors: 1},
		{name: "all failed", invocations: []error{errors.New("a"), errors.New("b")}, wantCount: 2, wantErrors: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, _ := capture(t)
			for _, e := range tt.invocations {
				err := e
				_ = rec.Track("PCD: Create Cinder Volume", func() error { return err })
			}

			got := rec.Snapshot()
			if len(got) != 1 {
				t.Fatalf("expected 1 step, got %d", len(got))
			}
			if got[0].Count != tt.wantCount {
				t.Errorf("Count = %d, want %d", got[0].Count, tt.wantCount)
			}
			if got[0].Errors != tt.wantErrors {
				t.Errorf("Errors = %d, want %d", got[0].Errors, tt.wantErrors)
			}
			if got[0].MaxMS > got[0].TotalMS {
				t.Errorf("MaxMS %d exceeds TotalMS %d", got[0].MaxMS, got[0].TotalMS)
			}
		})
	}
}

func TestTrackPropagatesError(t *testing.T) {
	rec, _ := capture(t)
	want := errors.New("attach failed")
	if got := rec.Track("PCD: Attach Volume to Helper VM", func() error { return want }); !errors.Is(got, want) {
		t.Fatalf("Track swallowed the error: got %v, want %v", got, want)
	}
}

func TestEmitsOneLinePerInvocation(t *testing.T) {
	rec, lines := capture(t)
	_ = rec.Track("vCenter: Take Snapshot", func() error { return nil })
	_ = rec.Track("vCenter: Take Snapshot", func() error { return errors.New("x") })

	if len(*lines) != 2 {
		t.Fatalf("expected 2 emitted lines, got %d: %v", len(*lines), *lines)
	}
	for i, want := range []string{"err=false", "err=true"} {
		line := (*lines)[i]
		if !strings.HasPrefix(line, LinePrefix) {
			t.Errorf("line %d missing %s prefix: %q", i, LinePrefix, line)
		}
		if !strings.Contains(line, `step="vCenter: Take Snapshot"`) {
			t.Errorf("line %d missing quoted step name: %q", i, line)
		}
		if !strings.Contains(line, "duration_ms=") {
			t.Errorf("line %d missing duration_ms: %q", i, line)
		}
		if !strings.Contains(line, want) {
			t.Errorf("line %d missing %s: %q", i, want, line)
		}
	}
}

func TestSnapshotPreservesFirstSeenOrder(t *testing.T) {
	rec, _ := capture(t)
	order := []string{"vCenter: Login", "PCD: Create Cinder Volume", "vCenter: Take Snapshot"}
	for _, step := range order {
		_ = rec.Track(step, func() error { return nil })
	}
	// Re-running an earlier step must not move it to the end.
	_ = rec.Track("vCenter: Login", func() error { return nil })

	got := rec.Snapshot()
	if len(got) != len(order) {
		t.Fatalf("expected %d steps, got %d", len(order), len(got))
	}
	for i, want := range order {
		if got[i].Step != want {
			t.Errorf("step %d = %q, want %q", i, got[i].Step, want)
		}
	}
}

// The Hot-Add path copies every disk from its own goroutine, so concurrent
// Start/record on the same recorder must not lose counts or race.
func TestConcurrentTrackIsSafe(t *testing.T) {
	rec, lines := capture(t)
	const goroutines, perGoroutine = 8, 25

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				done := rec.Start("HotAdd: nbdcopy")
				done(nil)
			}
		}()
	}
	wg.Wait()

	got := rec.Snapshot()
	if len(got) != 1 {
		t.Fatalf("expected 1 step, got %d", len(got))
	}
	if want := int64(goroutines * perGoroutine); got[0].Count != want {
		t.Errorf("Count = %d, want %d", got[0].Count, want)
	}
	if want := goroutines * perGoroutine; len(*lines) != want {
		t.Errorf("emitted %d lines, want %d", len(*lines), want)
	}
}

func TestEmitSummaryIsParseableJSON(t *testing.T) {
	rec, lines := capture(t)
	rec.SetDiskInfo(2, 214748364800, 107374182400)
	_ = rec.Track("vCenter: Take Snapshot", func() error { return nil })
	_ = rec.Track("PCD: Create Cinder Volume", func() error { return nil })
	_ = rec.Track("PCD: Create Cinder Volume", func() error { return nil })

	rec.EmitSummary(false)

	last := (*lines)[len(*lines)-1]
	payload, ok := strings.CutPrefix(last, SummaryPrefix+" ")
	if !ok {
		t.Fatalf("summary line missing %s prefix: %q", SummaryPrefix, last)
	}

	var got Summary
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("summary is not valid JSON: %v (%q)", err, payload)
	}

	if got.VM != "test-vm" {
		t.Errorf("VM = %q, want %q", got.VM, "test-vm")
	}
	if got.Method != "HotAdd" {
		t.Errorf("Method = %q, want %q", got.Method, "HotAdd")
	}
	if got.MigrationType != "cold" {
		t.Errorf("MigrationType = %q, want %q", got.MigrationType, "cold")
	}
	if got.DiskCount != 2 || got.DiskBytes != 214748364800 || got.AllocatedBytes != 107374182400 {
		t.Errorf("disk info round-trip failed: %+v", got)
	}
	if got.Failed {
		t.Error("Failed = true, want false")
	}
	if len(got.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(got.Steps))
	}
	if got.Steps[1].Count != 2 {
		t.Errorf("repeated step Count = %d, want 2", got.Steps[1].Count)
	}
}

func TestEmitSummaryMarksFailure(t *testing.T) {
	rec, lines := capture(t)
	rec.EmitSummary(true)

	payload := strings.TrimPrefix((*lines)[0], SummaryPrefix+" ")
	var got Summary
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("summary is not valid JSON: %v", err)
	}
	if !got.Failed {
		t.Error("Failed = false, want true on the failure path")
	}
}

func TestDefaultMethodIsNBD(t *testing.T) {
	rec := newWithEmitter("vm", "", "cold", func(string) {})
	if got := rec.Summarise(false).Method; got != "NBD" {
		t.Errorf("empty StorageCopyMethod = %q, want %q", got, "NBD")
	}
}

// New must tolerate a nil emitter — a missing log sink is never a reason to
// panic in the middle of a migration.
func TestNewWithNilEmitterDoesNotPanic(t *testing.T) {
	rec := New("vm", "HotAdd", "cold", nil)
	_ = rec.Track("vCenter: Take Snapshot", func() error { return nil })
	rec.EmitSummary(false)
	if got := rec.Snapshot(); len(got) != 1 {
		t.Fatalf("expected 1 step recorded, got %d", len(got))
	}
}

// A nil Recorder must behave as a no-op so call sites never need a guard.
func TestNilRecorderIsNoOp(t *testing.T) {
	var rec *Recorder

	want := errors.New("inner")
	if got := rec.Track("step", func() error { return want }); !errors.Is(got, want) {
		t.Errorf("nil Track did not run fn / propagate error: %v", got)
	}
	rec.Start("step")(nil)
	rec.SetDiskInfo(1, 2, 3)
	rec.EmitSummary(false)
	if got := rec.Snapshot(); got != nil {
		t.Errorf("nil Snapshot = %v, want nil", got)
	}
}
