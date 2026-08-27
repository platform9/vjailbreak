// Copyright © 2024 The vjailbreak authors

package nbd

import "testing"

func TestStartDebugStaleNFCSessions_NonPositiveCountIsNoop(t *testing.T) {
	for _, count := range []int{0, -1, -55} {
		got := StartDebugStaleNFCSessions(count, nil, "vcenter.example.com", "user", "pass", "thumb", "snap-1", "/vm/disk.vmdk")
		if got != nil {
			t.Errorf("StartDebugStaleNFCSessions(%d, ...) = %d sessions, want nil/no-op", count, len(got))
		}
	}
}

func TestStartDebugStaleNFCSessions_PositiveCountAttemptsAndDoesNotPanic(t *testing.T) {
	// nbdkit isn't installed in the unit test environment, so every attempt
	// to start a session is expected to fail before it ever gets to the
	// libnbd connect step - this exercises the "keep going and log" error
	// path rather than a happy path, and asserts the function never panics
	// and never returns more sessions than were requested.
	got := StartDebugStaleNFCSessions(3, nil, "vcenter.example.com", "user", "pass", "thumb", "snap-1", "/vm/disk.vmdk")
	if len(got) > 3 {
		t.Errorf("StartDebugStaleNFCSessions(3, ...) returned %d sessions, want at most 3", len(got))
	}

	// Whatever came back must tear down cleanly too.
	StopDebugStaleNFCSessions(got)
}

func TestStopDebugStaleNFCSessions_NilAndEmptyAreSafe(t *testing.T) {
	// Must not panic.
	StopDebugStaleNFCSessions(nil)
	StopDebugStaleNFCSessions([]*StaleNFCSession{})
}
