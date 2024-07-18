package nbd

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"libguestfs.org/libnbd"
)

//go:generate mockgen -source=../nbd/nbdops.go -destination=../nbd/nbdops_mock.go -package=nbd

type NBDOperations interface {
	StartNBDServer(server, username, password, thumbprint, snapref, file string) (NBDServer, error)
	StopNBDServer() error
	CopyDisk(dest string) error
	CopyChangedBlocks(changedAreas types.DiskChangeInfo, path string) error
}

type NBDServer struct {
	cmd     *exec.Cmd
	tmp_dir string
}

type BlockStatusData struct {
	Offset int64
	Length int64
	Flags  uint32
}

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

func verifynbdkit() error {
	// Verify that nbdkit is installed
	cmd := exec.Command("nbdkit", "--version")
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func StartNBDServer(vm *object.VirtualMachine, server, username, password, thumbprint, snapref, file string) (NBDServer, error) {
	// Start the NBD server
	// vm := ctx.Value("vm").(*object.VirtualMachine)
	tmp, err := os.MkdirTemp("", "nbdkit-")
	if err != nil {
		return NBDServer{}, err
	}

	socket := fmt.Sprintf("%s/nbdkit.sock", tmp)
	pidFile := fmt.Sprintf("%s/nbdkit.pid", tmp)

	cmd := exec.Command(
		"nbdkit",
		"--exit-with-parent",
		"--readonly",
		"--foreground",
		// "--verbose",
		fmt.Sprintf("--unix=%s", socket),
		fmt.Sprintf("--pidfile=%s", pidFile),
		"vddk",
		"libdir=/home/fedora/vmware-vix-disklib-distrib",
		fmt.Sprintf("server=%s", server),
		fmt.Sprintf("user=%s", username),
		fmt.Sprintf("password=%s", password),
		fmt.Sprintf("thumbprint=%s", thumbprint),
		"compression=skipz",
		fmt.Sprintf("vm=moref=%s", vm.Reference().Value),
		// fmt.Sprintf("snapshot=%s", snapref),
		file,
	)

	// Log the command
	cmdstring := ""
	for _, arg := range cmd.Args {

		if strings.Contains(arg, password) {
			cmdstring += fmt.Sprintf("password=[REDACTED] ")
		} else {
			cmdstring += fmt.Sprintf("%s ", arg)
		}
	}
	log.Printf("Executing %s\n", cmdstring)
	err = cmd.Start()
	if err != nil {
		return NBDServer{}, err
	}
	return NBDServer{
		cmd:     cmd,
		tmp_dir: tmp,
	}, nil
}

func (nbdserver NBDServer) StopNBDServer() error {
	err := nbdserver.cmd.Process.Kill()
	if err != nil {
		return err
	}
	os.RemoveAll(nbdserver.tmp_dir)
	return nil
}

func (nbdserver NBDServer) CopyDisk(dest string) error {
	// Copy the disk from source to destination
	progressRead, progressWrite, err := os.Pipe()
	if err != nil {
		return err
	}
	defer progressRead.Close()
	defer progressWrite.Close()

	cmd := exec.Command("nbdcopy", "--progress=3", "--target-is-zero", generateSockUrl(nbdserver.tmp_dir), dest)
	cmd.ExtraFiles = []*os.File{progressWrite}

	log.Println(cmd.String())
	go func() {
		scanner := bufio.NewScanner(progressRead)
		for scanner.Scan() {
			log.Printf("Progress: %s%%", scanner.Text())
		}
	}()
	err = cmd.Run()
	if err != nil {
		log.Println("Error running nbdcopy")
		return err
	}

	return nil
}

func getBlockStatus(handle *libnbd.Libnbd, extent types.DiskChangeExtent) []*BlockStatusData {
	var blocks []*BlockStatusData

	// Callback for libnbd.BlockStatus. Needs to modify blocks list above.
	updateBlocksCallback := func(metacontext string, nbdOffset uint64, extents []uint32, err *int) int {
		if nbdOffset > math.MaxInt64 {
			fmt.Errorf("Block status offset too big for conversion: 0x%x", nbdOffset)
			return -2
		}
		offset := int64(nbdOffset)

		if *err != 0 {
			fmt.Errorf("Block status callback error at offset %d: error code %d", offset, *err)
			return *err
		}
		if metacontext != "base:allocation" {
			log.Printf("Offset %d not base:allocation, ignoring", offset)
			return 0
		}
		if (len(extents) % 2) != 0 {
			fmt.Errorf("Block status entry at offset %d has unexpected length %d!", offset, len(extents))
			return -1
		}
		for i := 0; i < len(extents); i += 2 {
			length, flags := int64(extents[i]), extents[i+1]
			if blocks != nil {
				last := len(blocks) - 1
				lastBlock := blocks[last]
				lastFlags := lastBlock.Flags
				lastOffset := lastBlock.Offset + lastBlock.Length
				if lastFlags == flags && lastOffset == offset {
					// Merge with previous block
					blocks[last] = &BlockStatusData{
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
			fmt.Errorf("Error getting block status at offset %d, returning whole block instead. Error was: %v", lastOffset, err)
			return createWholeBlock()
		}
		last := len(blocks) - 1
		newOffset := blocks[last].Offset + blocks[last].Length
		if lastOffset == newOffset {
			log.Printf("No new block status data at offset %d, returning whole block.", newOffset)
			return createWholeBlock()
		}
		lastOffset = newOffset
	}

	return blocks
}

// pwrite writes the given byte buffer to the sink at the given offset
func pwrite(fd *os.File, buffer []byte, offset uint64) (int, error) {
	written, err := syscall.Pwrite(int(fd.Fd()), buffer, int64(offset))
	blocksize := len(buffer)
	if written < blocksize {
		log.Printf("Wrote less than blocksize (%d): %d", blocksize, written)
	}
	if err != nil {
		return -1, err
	}
	return written, err
}

// zeroRange fills the destination range with zero bytes
func zeroRange(fd *os.File, offset int64, length int64) error {
	punch := func(offset int64, length int64) error {
		log.Printf("Punching %d-byte hole at offset %d", length, offset)
		flags := uint32(unix.FALLOC_FL_PUNCH_HOLE | unix.FALLOC_FL_KEEP_SIZE)
		return syscall.Fallocate(int(fd.Fd()), flags, offset, length)
	}

	err := punch(offset, length)
	if err != nil {
		return err
	}

	if err != nil { // Fall back to regular pwrite
		log.Printf("Unable to zero range %d - %d on destination, falling back to pwrite: %v", offset, offset+length, err)
		err = nil
		count := int64(0)
		const blocksize = 16 << 20
		buffer := bytes.Repeat([]byte{0}, blocksize)
		for count < length {
			remaining := length - count
			if remaining < blocksize {
				buffer = bytes.Repeat([]byte{0}, int(remaining))
			}
			written, err := pwrite(fd, buffer, uint64(offset))
			if err != nil {
				log.Printf("Unable to write %d zeroes at offset %d: %v", length, offset, err)
				break
			}
			count += int64(written)
		}
	}

	return err
}

func copyRange(fd *os.File, handle *libnbd.Libnbd, block *BlockStatusData) error {
	if (block.Flags & (libnbd.STATE_ZERO | libnbd.STATE_HOLE)) != 0 {
		// log.Printf("Found a %d-byte %s at offset %d, filling destination with zeroes.", block.Length, skip, block.Offset)
		err := zeroRange(fd, block.Offset, block.Length)
		// updateProgress(int(block.Length))
		return err
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
			log.Printf("Error reading from source at offset %d: %v", offset, err)
			return err
		}

		_, err = pwrite(fd, buffer, uint64(offset))
		if err != nil {
			log.Printf("Failed to write data block at offset %d to local file: %v", block.Offset, err)
			return err
		}
		// log.Printf("Copied %d bytes at offset %d", length, offset)

		// updateProgress(written)
		count += int64(length)
	}
	return nil
}

func (nbdserver NBDServer) CopyChangedBlocks(changedAreas types.DiskChangeInfo, path string) error {
	// Copy the changed blocks from source to destination
	handle, err := libnbd.Create()
	if err != nil {
		return err
	}
	err = handle.AddMetaContext("base:allocation")
	if err != nil {
		return err
	}
	err = handle.ConnectUri(generateSockUrl(nbdserver.tmp_dir))
	if err != nil {
		return err
	}
	defer handle.Close()

	// log.Println("Opening File")
	fd, err := os.OpenFile(path, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	// log.Println("File Opened")

	defer fd.Close()

	totalsize := int64(0)
	copiedsize := int64(0)

	for _, extent := range changedAreas.ChangedArea {
		totalsize += extent.Length
	}
	for _, extent := range changedAreas.ChangedArea {
		blocks := getBlockStatus(handle, extent)
		for _, block := range blocks {
			err := copyRange(fd, handle, block)
			if err != nil {
				return err
			}
		}
		copiedsize += extent.Length
		// if (idx+1)%10 == 0 {
		log.Printf("Progress: %.2f%%\n", float64(copiedsize)/float64(totalsize)*100.0)
		// }
	}
	// log.Printf("Copied %d/%d bytes", copiedsize, totalsize)
	return nil
}

func generateSockUrl(tmp_dir string) string {
	return fmt.Sprintf("nbd+unix:///?socket=%s/nbdkit.sock", tmp_dir)
}
