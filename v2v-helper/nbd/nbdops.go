// Copyright © 2024 The vjailbreak authors

package nbd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"

	"golang.org/x/sys/unix"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"libguestfs.org/libnbd"
)

//go:generate mockgen -source=../nbd/nbdops.go -destination=../nbd/nbdops_mock.go -package=nbd

type NBDOperations interface {
	StartNBDServer(vm *object.VirtualMachine, server, username, password, thumbprint, snapref, file string, progchan chan string) error
	StopNBDServer() error
	CopyDisk(ctx context.Context, dest string, diskindex int) error
	CopyChangedBlocks(ctx context.Context, changedAreas types.DiskChangeInfo, path string) error
	GetProgress() (int64, int64, time.Duration)
}

type NBDServer struct {
	cmd          *exec.Cmd
	tmp_dir      string
	progresschan chan string
	TotalSize    int64
	StartTime    time.Time
	CopiedSize   int64
	Duration     time.Duration
}

type BlockStatusData struct {
	Offset int64
	Length int64
	Flags  uint32
}

// copiedsize,totalsize,start time,current time

const MaxChunkSize = 64 * 1024 * 1024

// MaxBlockStatusLength limits the maximum block status request size to 2GB
const MaxBlockStatusLength = (2 << 30)

// MaxPreadLengthESX limits individual VDDK data block transfers to 23MB.
// Larger block sizes fail immediately.
const MaxPreadLengthESX = (23 << 20)

// MaxPreadLengthVC limits indidivual VDDK data block transfers to 2MB only when
// connecting to vCenter. With vCenter endpoints, multiple simultaneous importer
// pods with larger read sizes cause allocation failures on the server, and the
// imports start to fail:
//
//	"NfcFssrvrProcessErrorMsg: received NFC error 5 from server:
//	 Failed to allocate the requested 24117272 bytes"
const MaxPreadLengthVC = (2 << 20)

// MaxPreadLength is the maxmimum read size to request from VMware. Default to
// the larger option, and reduce it in createVddkDataSource when connecting to
// vCenter endpoints.
var MaxPreadLength = MaxPreadLengthESX

// Request blocks one at a time from libnbd
var fixedOptArgs = libnbd.BlockStatusOptargs{
	Flags:    libnbd.CMD_FLAG_REQ_ONE,
	FlagsSet: true,
}

// HandlePoolSize is the number of libnbd handles created up-front and shared
// across worker goroutines. libnbd serializes commands per handle, so real
// pipelining only happens at the granularity of the pool size.
const HandlePoolSize = 8

// SubRangeSize is the chunk size used to split a single large data block into
// independently-fetched sub-ranges that can run on different handles in parallel.
const SubRangeSize = 256 << 20 // 256 MiB

// ExtentCoalesceGap is the maximum gap (in bytes) between two adjacent
// CBT-reported extents that we will merge into a single extent before
// dispatching. The gap region is re-checked with BlockStatus and any holes
// inside it are punched cheaply, so the only cost is one BlockStatus call
// instead of many tiny ones.
const ExtentCoalesceGap = 16 << 20 // 16 MiB

// ExtentWorkerCount caps the number of concurrent extent workers. Bytes-level
// parallelism is provided by the handle pool; this just bounds goroutine count
// so that workloads with millions of fragmented extents don't blow up memory.
const ExtentWorkerCount = 32

// handlePool is a fixed-size pool of pre-connected libnbd handles. Workers
// Acquire a handle for the duration of an I/O operation and Release it when
// done. The capacity of the channel == HandlePoolSize, so Acquire blocks
// when all handles are in use, naturally throttling parallel I/O to the
// pool size.
type handlePool struct {
	handles chan *libnbd.Libnbd
	all     []*libnbd.Libnbd
	size    int
}

func newHandlePool(size int, sockUrl string) (*handlePool, error) {
	pool := &handlePool{
		handles: make(chan *libnbd.Libnbd, size),
		all:     make([]*libnbd.Libnbd, 0, size),
		size:    size,
	}
	for handleIdx := 0; handleIdx < size; handleIdx++ {
		handle, err := libnbd.Create()
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("failed to create libnbd handle %d: %v", handleIdx, err)
		}
		if err := handle.AddMetaContext("base:allocation"); err != nil {
			handle.Close()
			pool.Close()
			return nil, fmt.Errorf("failed to add meta context on handle %d: %v", handleIdx, err)
		}
		if err := handle.ConnectUri(sockUrl); err != nil {
			handle.Close()
			pool.Close()
			// libnbd's own error rarely says more than "connection refused" -
			// the actual reason (DNS failure, port 902 blocked, bad
			// thumbprint, etc.) is in nbdkit's own stdout/stderr, captured in
			// the dedicated "nbd" debug log. Pull the most relevant line out
			// of it and fold it into the returned error.
			if reason := nbdFailureReasonFromLatestLog(); reason != "" {
				return nil, fmt.Errorf("failed to connect handle %d: %v: %s", handleIdx, err, reason)
			}
			return nil, fmt.Errorf("failed to connect handle %d: %v", handleIdx, err)
		}
		pool.all = append(pool.all, handle)
		pool.handles <- handle
	}
	return pool, nil
}

// Acquire returns a handle from the pool, blocking until one is available
// or the context is cancelled.
func (pool *handlePool) Acquire(ctx context.Context) (*libnbd.Libnbd, error) {
	select {
	case handle := <-pool.handles:
		return handle, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Release returns a handle to the pool.
func (pool *handlePool) Release(handle *libnbd.Libnbd) {
	if handle == nil {
		return
	}
	pool.handles <- handle
}

// Close tears down every handle. Caller must guarantee no more Acquire calls.
func (pool *handlePool) Close() {
	for _, handle := range pool.all {
		if handle != nil {
			_ = handle.Close()
		}
	}
}

// coalesceExtents sorts extents by offset and merges any pair whose gap is
// <= maxGap. Inside a merged extent, getBlockStatus will discover the gap as
// a hole and zeroRange will punch it; the only cost is one larger BlockStatus
// call versus many smaller ones. Returns a new slice; input is not mutated.
func coalesceExtents(extents []types.DiskChangeExtent, maxGap int64) []types.DiskChangeExtent {
	if len(extents) <= 1 {
		return extents
	}
	sorted := make([]types.DiskChangeExtent, len(extents))
	copy(sorted, extents)
	sort.Slice(sorted, func(leftIdx, rightIdx int) bool { return sorted[leftIdx].Start < sorted[rightIdx].Start })

	coalesced := make([]types.DiskChangeExtent, 0, len(sorted))
	coalesced = append(coalesced, sorted[0])
	for _, extent := range sorted[1:] {
		lastExtent := &coalesced[len(coalesced)-1]
		lastExtentEnd := lastExtent.Start + lastExtent.Length
		extentEnd := extent.Start + extent.Length
		if extent.Start-lastExtentEnd <= maxGap {
			if extentEnd > lastExtentEnd {
				lastExtent.Length = extentEnd - lastExtent.Start
			}
		} else {
			coalesced = append(coalesced, extent)
		}
	}
	return coalesced
}

func (nbdserver *NBDServer) StartNBDServer(vm *object.VirtualMachine, server, username, password, thumbprint, snapref, file string, progchan chan string) error {
	server = strings.TrimRight(server, "/")

	tmp_dir, err := os.MkdirTemp("", "nbdkit-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %v", err)
	}
	// Create the configuration file
	// Ref: https://tecblog.au.de/veeam-v12-12-1-double-your-nbd-backup-performance/
	configFile := "/home/fedora/vddk.conf"
	configContent := `vixDiskLib.nfcAio.Session.BufSizeIn64KB=16
vixDiskLib.nfcAio.Session.BufCount=4`
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		return err
	}

	socket := fmt.Sprintf("%s/nbdkit.sock", tmp_dir)
	pidFile := fmt.Sprintf("%s/nbdkit.pid", tmp_dir)

	cmd := exec.Command(
		"nbdkit",
		"--exit-with-parent",
		"--readonly",
		"--foreground",
		fmt.Sprintf("--unix=%s", socket),
		fmt.Sprintf("--pidfile=%s", pidFile),
		"--verbose",
		"-D vddk.datapath=0",
		"-D nbdkit.backend.datapath=0",
		"vddk",
		"libdir=/home/fedora/vmware-vix-disklib-distrib",
		fmt.Sprintf("server=%s", server),
		fmt.Sprintf("user=%s", username),
		fmt.Sprintf("password=%s", password),
		fmt.Sprintf("thumbprint=%s", thumbprint),
		"compression=fastlz",
		"config=/home/fedora/vddk.conf",
		"transports=file:nbdssl:nbd",
		fmt.Sprintf("vm=moref=%s", vm.Reference().Value),
		fmt.Sprintf("snapshot=%s", snapref),
		file,
	)

	// Log the command with password redacted
	cmdstring := ""
	for _, arg := range cmd.Args {
		if strings.Contains(arg, password) {
			cmdstring += "password=[REDACTED] "
		} else {
			cmdstring += fmt.Sprintf("%s ", arg)
		}
	}

	utils.AddDebugOutputToFileWithCommandCategory(cmd, cmdstring, utils.LogCategoryNBD, password)

	utils.PrintLog(fmt.Sprintf("Executing %s\n", cmdstring))
	err = cmd.Start()
	if err != nil {
		// Close log file if nbdkit failed to start
		utils.CloseLogFile(cmd)
		return fmt.Errorf("failed to start nbdkit: %v", err)
	}
	nbdserver.cmd = cmd
	nbdserver.tmp_dir = tmp_dir
	nbdserver.progresschan = progchan
	return nil
}

func (nbdserver *NBDServer) StopNBDServer() error {
	err := nbdserver.cmd.Process.Kill()
	if err != nil {
		return fmt.Errorf("failed to kill nbdkit: %v", err)
	}
	os.RemoveAll(nbdserver.tmp_dir)
	return nil
}

func (nbdserver *NBDServer) CopyDisk(ctx context.Context, dest string, diskindex int) error {
	// Copy the disk from source to destination
	progressRead, progressWrite, err := os.Pipe()
	if err != nil {
		return errors.Wrapf(err, "failed to create pipe")
	}
	defer progressRead.Close()
	defer progressWrite.Close()

	cmd := exec.CommandContext(ctx, "nbdcopy", "--progress=3", "--target-is-zero", generateSockUrl(nbdserver.tmp_dir), dest)
	cmd.ExtraFiles = []*os.File{progressWrite}

	cmdString := cmd.String()
	utils.PrintLog(fmt.Sprintf("Executing %s\n", cmdString))
	go func() {
		scanner := bufio.NewScanner(progressRead)

		lastLoggedProgress := -1
		const logInterval = 5
		lastChannelProgress := 0

		for scanner.Scan() {
			progressInt, _, err := utils.ParseFraction(scanner.Text())
			if err != nil {
				utils.PrintLog(fmt.Sprintf("Error converting progress percent to int: %v", err))
				continue
			}
			msg := fmt.Sprintf("Copying disk %d, Completed: %d%%", diskindex, progressInt)

			if (progressInt == 0 && lastLoggedProgress != 0) || progressInt == 100 || (progressInt > lastLoggedProgress && progressInt%logInterval == 0) {
				utils.PrintLog(msg)
				lastLoggedProgress = progressInt
			}

			if lastChannelProgress <= progressInt-10 {
				nbdserver.progresschan <- msg
				lastChannelProgress = progressInt
			}
		}
	}()
	// Use the helper function with command string to ensure log file is closed after command execution.
	// nbdcopy output goes into the same dedicated nbd log file as nbdkit's.
	err = utils.RunCommandWithLogFileRedactedCategory(cmd, cmdString, utils.LogCategoryNBD)
	if err != nil {
		// retry once with debug enabled, to get more details
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			// nbdcopy/nbdkit's own error is rarely more than "exit status 1".
			// The actionable root cause (ESXi DNS resolution failure, port
			// 902 blocked, bad thumbprint, NFC resource exhaustion, etc.) is
			// only in the dedicated nbd debug log, so pull the most relevant
			// line out of it and fold it into the returned error.
			if reason := nbdFailureReasonFromLatestLog(); reason != "" {
				return errors.Wrapf(err, "failed to run nbdcopy: %s", reason)
			}
			return errors.Wrapf(err, "failed to run nbdcopy")
		}
	}
	return nil
}

// nbdFailureReasonFromLatestLog looks up the debug log file that was just
// written for the current migration's nbd-category commands (nbdkit,
// nbdcopy) and extracts a concise, human-readable root-cause line from it.
// Returns "" if the migration name or log file can't be resolved, or no
// error line is found - in which case callers should fall back to the raw
// process error.
func nbdFailureReasonFromLatestLog() string {
	migrationName, err := utils.GetMigrationObjectName()
	if err != nil {
		return ""
	}

	logPath, err := utils.GetLatestLogFilePath(migrationName, utils.LogCategoryNBD)
	if err != nil {
		return ""
	}

	return utils.ExtractNBDFailureReason(logPath)
}

func getBlockStatus(handle *libnbd.Libnbd, extent types.DiskChangeExtent) []*BlockStatusData {
	var blocks []*BlockStatusData

	// Callback for libnbd.BlockStatus. Needs to modify blocks list above.
	updateBlocksCallback := func(metacontext string, nbdOffset uint64, extents []uint32, err *int) int {
		if nbdOffset > math.MaxInt64 {
			utils.PrintLog(fmt.Sprintf("Block status offset too big for conversion: 0x%x", nbdOffset))
			return -2
		}
		offset := int64(nbdOffset)

		if *err != 0 {
			utils.PrintLog(fmt.Sprintf("Block status callback error at offset %d: error code %d", offset, *err))
			return *err
		}
		if metacontext != "base:allocation" {
			utils.PrintLog(fmt.Sprintf("Offset %d not base:allocation, ignoring", offset))
			return 0
		}
		if (len(extents) % 2) != 0 {
			utils.PrintLog(fmt.Sprintf("Block status entry at offset %d has unexpected length %d!", offset, len(extents)))
			return -1
		}
		for extentPairIdx := 0; extentPairIdx < len(extents); extentPairIdx += 2 {
			length, flags := int64(extents[extentPairIdx]), extents[extentPairIdx+1]
			if blocks != nil {
				lastBlockIdx := len(blocks) - 1
				lastBlock := blocks[lastBlockIdx]
				lastFlags := lastBlock.Flags
				lastOffset := lastBlock.Offset + lastBlock.Length
				if lastFlags == flags && lastOffset == offset {
					// Merge with previous block
					blocks[lastBlockIdx] = &BlockStatusData{
						Offset: lastBlock.Offset,
						Length: lastBlock.Length + length,
						Flags:  lastFlags,
					}
				} else {
					blocks = append(blocks, &BlockStatusData{Offset: offset, Length: length, Flags: flags})
				}
			} else {
				blocks = append(blocks, &BlockStatusData{Offset: offset, Length: length, Flags: flags})
			}
			offset += length
		}
		return 0
	}

	if extent.Length < 1024*1024 {
		blocks = append(blocks, &BlockStatusData{
			Offset: extent.Start,
			Length: extent.Length,
			Flags:  0})
		return blocks
	}

	lastOffset := extent.Start
	endOffset := extent.Start + extent.Length
	for lastOffset < endOffset {
		var length int64
		missingLength := endOffset - lastOffset
		if missingLength > (MaxBlockStatusLength) {
			length = MaxBlockStatusLength
		} else {
			length = missingLength
		}
		createWholeBlock := func() []*BlockStatusData {
			block := &BlockStatusData{
				Offset: extent.Start,
				Length: extent.Length,
				Flags:  0,
			}
			blocks = []*BlockStatusData{block}
			return blocks
		}
		err := handle.BlockStatus(uint64(length), uint64(lastOffset), updateBlocksCallback, &fixedOptArgs)
		if err != nil {
			utils.PrintLog(fmt.Sprintf("Error getting block status at offset %d, returning whole block instead. Error was: %v", lastOffset, err))
			return createWholeBlock()
		}
		lastBlockIdx := len(blocks) - 1
		newOffset := blocks[lastBlockIdx].Offset + blocks[lastBlockIdx].Length
		if lastOffset == newOffset {
			utils.PrintLog(fmt.Sprintf("No new block status data at offset %d, returning whole block.", newOffset))
			return createWholeBlock()
		}
		lastOffset = newOffset
	}

	return blocks
}

// pwrite writes the given byte buffer to the sink at the given offset
func pwrite(fd *os.File, buffer []byte, offset uint64) (int, error) {
	blocksize := len(buffer)
	written, err := syscall.Pwrite(int(fd.Fd()), buffer, int64(offset))
	if err != nil {
		return -1, errors.Wrapf(err, "failed to write %d bytes at offset %d", blocksize, offset)
	}
	if written < blocksize {
		utils.PrintLog(fmt.Sprintf("Wrote less than blocksize (%d): %d", blocksize, written))
	}
	return written, nil
}

// zeroRange fills the destination range with zero bytes
func zeroRange(fd *os.File, offset int64, length int64) error {
	punch := func(offset int64, length int64) error {
		utils.PrintLog(fmt.Sprintf("Punching %d-byte hole at offset %d", length, offset))
		flags := uint32(unix.FALLOC_FL_PUNCH_HOLE | unix.FALLOC_FL_KEEP_SIZE)
		return syscall.Fallocate(int(fd.Fd()), flags, offset, length)
	}

	punchErr := punch(offset, length)
	if punchErr == nil {
		return nil
	}
	// Fall back to writing zeros directly. This is the slow path; it
	// triggers on filesystems that don't support FALLOC_FL_PUNCH_HOLE
	// (some NFS configurations, tmpfs, certain block-backed targets).
	utils.PrintLog(fmt.Sprintf("Failed to punch hole at offset %d, falling back to pwrite: %v", offset, punchErr))
	utils.PrintLog(fmt.Sprintf("Unable to zero range %d - %d on destination, falling back to pwrite: %v", offset, offset+length, punchErr))

	count := int64(0)
	const blocksize = 16 << 20
	buffer := bytes.Repeat([]byte{0}, blocksize)
	for count < length {
		remaining := length - count
		if remaining < blocksize {
			buffer = bytes.Repeat([]byte{0}, int(remaining))
		}
		// Use offset+count, not offset, so the writes actually advance
		// through the range instead of repeatedly overwriting the first
		// chunk. Returning the wrapped error (instead of breaking and
		// returning nil) makes the failure visible to callers — without
		// this, a partial zero of the destination would be silently
		// reported as a successful migration.
		written, err := pwrite(fd, buffer, uint64(offset+count))
		if err != nil {
			return errors.Wrapf(err, "zeroRange fallback pwrite failed at offset %d (%d/%d bytes written before failure)", offset+count, count, length)
		}
		count += int64(written)
	}
	return nil
}

func copyRange(fd *os.File, handle *libnbd.Libnbd, block *BlockStatusData) error {
	isZeroOrHole := (block.Flags & (libnbd.STATE_ZERO | libnbd.STATE_HOLE)) != 0

	if isZeroOrHole {
		err := zeroRange(fd, block.Offset, block.Length)
		if err != nil {
			return fmt.Errorf("failed to zero range at offset %d: %v", block.Offset, err)
		}
		return nil
	}

	buffer := bytes.Repeat([]byte{0}, MaxPreadLength)
	count := int64(0)
	for count < block.Length {
		if block.Length-count < int64(MaxPreadLength) {
			buffer = bytes.Repeat([]byte{0}, int(block.Length-count))
		}
		length := len(buffer)

		offset := block.Offset + count
		err := handle.Pread(buffer, uint64(offset), nil)
		if err != nil {
			return fmt.Errorf("error reading from source at offset %d: %v", offset, err)
		}

		_, err = pwrite(fd, buffer, uint64(offset))
		if err != nil {
			return fmt.Errorf("failed to write data block at offset %d to local file: %v", block.Offset, err)
		}
		count += int64(length)
	}
	return nil
}

// copyBlockParallel handles a single BlockStatusData. For zero/hole blocks it
// punches a hole locally (no source read needed). For data blocks larger than
// SubRangeSize it splits the block into sub-ranges and dispatches them across
// the handle pool so that multiple Preads run in flight concurrently — which
// is the only way to exceed single-stream VDDK throughput. Smaller blocks
// take a single handle from the pool and run copyRange as before.
func copyBlockParallel(ctx context.Context, fd *os.File, pool *handlePool, block *BlockStatusData) error {
	isZeroOrHole := (block.Flags & (libnbd.STATE_ZERO | libnbd.STATE_HOLE)) != 0

	if isZeroOrHole {
		return zeroRange(fd, block.Offset, block.Length)
	}

	// Small block: keep the original single-handle path to avoid sub-range
	// overhead when there's nothing to gain.
	if block.Length <= int64(SubRangeSize) {
		handle, err := pool.Acquire(ctx)
		if err != nil {
			return fmt.Errorf("acquire handle for small block at offset %d: %v", block.Offset, err)
		}
		defer pool.Release(handle)
		return copyRange(fd, handle, block)
	}

	// Plan sub-ranges. Each sub-range is treated as a fresh data block so
	// copyRange's inner loop drives Preads sequentially within the sub-range
	// while peers run on other handles in parallel.
	var subRanges []*BlockStatusData
	blockEnd := block.Offset + block.Length
	for subRangeOffset := block.Offset; subRangeOffset < blockEnd; subRangeOffset += int64(SubRangeSize) {
		subRangeEnd := subRangeOffset + int64(SubRangeSize)
		if subRangeEnd > blockEnd {
			subRangeEnd = blockEnd
		}
		subRanges = append(subRanges, &BlockStatusData{
			Offset: subRangeOffset,
			Length: subRangeEnd - subRangeOffset,
			Flags:  0, // we already know this region is data
		})
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(subRanges))
	for subRangeIdx, subRange := range subRanges {
		wg.Add(1)
		go func(subRangeIdx int, subRange *BlockStatusData) {
			defer wg.Done()
			handle, err := pool.Acquire(ctx)
			if err != nil {
				errCh <- fmt.Errorf("acquire handle for sub-range %d: %v", subRangeIdx, err)
				return
			}
			defer pool.Release(handle)
			if err := copyRange(fd, handle, subRange); err != nil {
				errCh <- fmt.Errorf("sub-range %d at offset %d failed: %v", subRangeIdx, subRange.Offset, err)
				return
			}
		}(subRangeIdx, subRange)
	}
	wg.Wait()
	close(errCh)
	if err, ok := <-errCh; ok && err != nil {
		return err
	}
	return nil
}

func (nbdserver *NBDServer) GetProgress() (int64, int64, time.Duration) {
	return nbdserver.CopiedSize, nbdserver.TotalSize, nbdserver.Duration
}

func (nbdserver *NBDServer) CopyChangedBlocks(ctx context.Context, changedAreas types.DiskChangeInfo, path string) error {
	// Coalesce CBT-reported extents to amortize per-extent overhead. Holes
	// inside a coalesced range will be discovered by getBlockStatus and
	// punched cheaply via fallocate, so there's no read amplification beyond
	// the chosen gap threshold.
	coalesced := coalesceExtents(changedAreas.ChangedArea, int64(ExtentCoalesceGap))

	// Build the handle pool that all workers will share.
	pool, err := newHandlePool(HandlePoolSize, generateSockUrl(nbdserver.tmp_dir))
	if err != nil {
		return fmt.Errorf("failed to build handle pool: %v", err)
	}

	fd, err := os.OpenFile(path, os.O_WRONLY, 0644)
	if err != nil {
		pool.Close()
		return fmt.Errorf("failed to open file: %v", err)
	}

	// Derived context so the first worker error can fan-out cancellation to
	// every goroutine we spawn (workers, sub-range goroutines, feeder, progress
	// aggregator) without disturbing the caller's ctx. cancel() runs first in
	// the deferred cleanup as a safety net; the *invariant* the function body
	// enforces is that pool.Close() / fd.Close() never execute while any
	// goroutine could still be using them.
	copyCtx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		pool.Close()
		fd.Close()
	}()

	totalsize := int64(0)
	for _, extent := range coalesced {
		totalsize += extent.Length
	}

	incrementalcopyprogress := make(chan int64, len(coalesced))
	errorChan := make(chan error, ExtentWorkerCount)
	doneChan := make(chan struct{})
	progressDone := make(chan struct{})

	maxRetries, capInterval := utils.GetRetryLimits()

	// Progress aggregator. Drains incrementalcopyprogress until it is closed
	// (which the function body only does after all workers have returned).
	go func() {
		defer close(progressDone)
		copiedsize := int64(0)
		lastLoggedPct := -1
		const logInterval = 5
		startTime := time.Now()
		nbdserver.StartTime = startTime
		nbdserver.TotalSize = totalsize
		for progress := range incrementalcopyprogress {
			copiedsize += progress
			nbdserver.CopiedSize = copiedsize
			nbdserver.Duration = time.Since(startTime)

			currentPct := 0
			if totalsize > 0 {
				currentPct = int(float64(copiedsize) / float64(totalsize) * 100.0)
			}
			prog := fmt.Sprintf("Progress: %d%%", currentPct)

			if (currentPct == 0 && lastLoggedPct != 0) || currentPct == 100 || (currentPct > lastLoggedPct && currentPct%logInterval == 0) {
				utils.PrintLog(prog)
				lastLoggedPct = currentPct
			}
			// Non-blocking send: progresschan is unbuffered. If the consumer
			// is not reading right now, drop the update — progress is monotonic
			// and frequent, so the next update will get through. The cancel
			// case ensures we exit promptly on shutdown.
			select {
			case nbdserver.progresschan <- prog:
			case <-copyCtx.Done():
				return
			default:
			}
		}
	}()

	// Fixed-size extent worker pool fed by an extent channel. Bounds goroutine
	// fan-out for fragmented workloads while still letting bytes-level
	// parallelism happen inside copyBlockParallel via the handle pool.
	extentCh := make(chan types.DiskChangeExtent)
	go func() {
		defer close(extentCh)
		for _, extent := range coalesced {
			select {
			case extentCh <- extent:
			case <-copyCtx.Done():
				return
			}
		}
	}()

	workerCount := ExtentWorkerCount
	if workerCount > len(coalesced) && len(coalesced) > 0 {
		workerCount = len(coalesced)
	}
	if workerCount < 1 {
		workerCount = 1
	}

	var wg sync.WaitGroup
	for workerIdx := 0; workerIdx < workerCount; workerIdx++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for extent := range extentCh {
				// getBlockStatus needs a handle for one BlockStatus walk. Hold it
				// for that walk only, then release before doing the (longer) reads.
				handle, err := pool.Acquire(copyCtx)
				if err != nil {
					// copyCtx was cancelled; the originating error is already
					// in errorChan. Exit quietly.
					return
				}
				blocks := getBlockStatus(handle, extent)
				pool.Release(handle)

				retries := uint64(0)
				waitTime := 1 * time.Minute
				for blockIdx := 0; blockIdx < len(blocks); {
					if err := copyBlockParallel(copyCtx, fd, pool, blocks[blockIdx]); err != nil {
						// If we were cancelled (peer worker errored), don't
						// log a confusing failure or burn retries — just exit.
						if copyCtx.Err() != nil {
							return
						}
						utils.PrintLog(fmt.Sprintf("Failed to copy block: %v | attempt %d", err, retries))
						retries++
						if retries >= maxRetries {
							// errorChan has capacity ExtentWorkerCount so this
							// send shouldn't block, but be defensive: use a
							// non-blocking select so a future buffer-size
							// reduction can't deadlock the worker.
							select {
							case errorChan <- errors.Wrap(err, "failed to copy changed blocks, exceeded retries"):
							default:
							}
							return
						}
						select {
						case <-copyCtx.Done():
							return
						case <-time.After(waitTime):
							waitTime = waitTime * 2
							if waitTime > capInterval {
								waitTime = capInterval
							}
							continue
						}
					} else {
						blockIdx++
						retries = uint64(0)
					}
				}

				select {
				case <-copyCtx.Done():
					return
				case incrementalcopyprogress <- extent.Length:
				}
			}
		}(workerIdx)
	}

	go func() {
		wg.Wait()
		close(doneChan)
	}()

	// Wait for either clean completion or the first error. On error we
	// cancel() and wait for every worker (and its sub-range goroutines via
	// copyBlockParallel's internal wg.Wait()) to return before tearing down
	// the handle pool and file descriptor in the deferred cleanup.
	var firstErr error
	select {
	case <-doneChan:
		// Happy path: every worker exited cleanly. Drain errorChan to catch
		// the race where the last errored worker's wg.Done() fired in the
		// same instant doneChan closed.
		select {
		case firstErr = <-errorChan:
		default:
		}
	case firstErr = <-errorChan:
		cancel()
		<-doneChan
	}

	// All workers have returned. It is now safe to close the progress channel
	// (no more senders) and wait for the aggregator to drain whatever was
	// buffered and exit.
	close(incrementalcopyprogress)
	<-progressDone

	return firstErr
}

func generateSockUrl(tmp_dir string) string {
	return fmt.Sprintf("nbd+unix:///?socket=%s/nbdkit.sock", tmp_dir)
}
