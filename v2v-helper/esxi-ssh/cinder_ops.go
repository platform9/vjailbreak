// Copyright © 2024 The vjailbreak authors

package esxissh

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage/pure"
	"k8s.io/klog/v2"
)

// CinderConversionResult contains the result of converting a VMDK to Cinder volume
type CinderConversionResult struct {
	SourceVMDK       string
	TargetVMDK       string
	SourceNAA        string
	TargetNAA        string
	OriginalVolName  string
	CinderVolName    string
	CinderVolumeID   string
	CloneDuration    time.Duration
	TotalDuration    time.Duration
	ConversionStatus string
}

// ConvertVMDKToCinder performs the complete VMDK to Cinder conversion workflow:
// 1. Get source VMDK size
// 2. Create new Pure volume with Cinder name: volume-<uuid>-cinder
// 3. Present volume to ESXi host and rescan storage
// 4. Create VMFS datastore on the new Pure volume
// 5. VAAI XCOPY clone source VMDK to new datastore (hardware accelerated, 3-5 seconds)
// 6. Volume is ready for OpenStack - already has Cinder name!
//
// This creates a dynamic Pure volume + datastore - entire process takes ~10-15 seconds
// No rename needed - volume created with correct Cinder name from the start
func (c *Client) ConvertVMDKToCinder(
	ctx context.Context,
	sourceVMDK string,
	cinderVolumeID string,
	esxiHostname string,
	pureProvider *pure.PureStorageProvider,
) (*CinderConversionResult, error) {

	overallStart := time.Now()
	cinderVolName := fmt.Sprintf("volume-%s-cinder", cinderVolumeID)
	datastoreName := fmt.Sprintf("cinder-%s", cinderVolumeID[:8]) // Short datastore name

	result := &CinderConversionResult{
		SourceVMDK:     sourceVMDK,
		CinderVolumeID: cinderVolumeID,
		CinderVolName:  cinderVolName,
	}

	klog.Infof("Starting VMDK to Cinder conversion: %s → %s", sourceVMDK, cinderVolName)

	// Step 1: Get source VMDK size
	klog.Infof("Step 1: Getting source VMDK size...")
	sourceSize, err := c.GetVMDKSize(sourceVMDK)
	if err != nil {
		return nil, fmt.Errorf("failed to get source VMDK size: %w", err)
	}
	klog.Infof("Source VMDK size: %d bytes (%.2f GB)", sourceSize, float64(sourceSize)/(1024*1024*1024))

	// Step 2: Create Pure volume with Cinder name (or use existing)
	klog.Infof("Step 2: Creating Pure volume %s (%d bytes)...", cinderVolName, sourceSize)
	err = pureProvider.CreateVolume(cinderVolName, sourceSize)
	if err != nil {
		// Check if volume already exists
		if strings.Contains(err.Error(), "already exists") {
			klog.Infof("Volume %s already exists, will use existing volume", cinderVolName)
		} else {
			return nil, fmt.Errorf("failed to create Pure volume: %w", err)
		}
	} else {
		klog.Infof("✓ Created Pure volume: %s", cinderVolName)
	}
	result.OriginalVolName = cinderVolName

	// Step 3: Get ESXi host IQN and map volume to host
	klog.Infof("Step 3: Getting ESXi host IQN...")
	hostIQN, err := c.GetHostIQN()
	if err != nil {
		klog.Warningf("Could not get host IQN: %v - will skip automatic volume mapping", err)
	} else {
		klog.Infof("ESXi Host IQN: %s", hostIQN)

		// Map volume to ESXi host using IQN
		klog.Infof("Mapping volume %s to ESXi host...", cinderVolName)
		hostContext, err := pureProvider.CreateOrUpdateInitiatorGroup(esxiHostname, []string{hostIQN})
		if err != nil {
			klog.Warningf("Could not create initiator group: %v", err)
		} else {
			_, err = pureProvider.MapVolumeToGroup(esxiHostname, storage.Volume{Name: cinderVolName}, hostContext)
			if err != nil {
				klog.Warningf("Could not map volume to host: %v", err)
			} else {
				klog.Infof("✓ Mapped volume to ESXi host")
			}
		}
	}

	// Step 4: Rescan ESXi storage to detect new Pure volume
	klog.Infof("Step 4: Rescanning ESXi storage...")
	err = c.RescanStorage()
	if err != nil {
		klog.Warningf("Storage rescan warning: %v", err)
	}

	// Wait longer for ESXi to fully detect and process the new volume
	klog.Infof("Waiting for ESXi to detect new volume...")
	time.Sleep(10 * time.Second) // Give ESXi time to detect and process the volume

	// Step 5: Get NAA of new Cinder volume
	klog.Infof("Step 5: Resolving Cinder volume to NAA...")
	cinderVol, err := pureProvider.ResolveCinderVolumeToLUN(cinderVolumeID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Cinder volume to NAA: %w", err)
	}
	result.TargetNAA = cinderVol.NAA
	klog.Infof("✓ Cinder volume NAA: %s", result.TargetNAA)

	// Step 6: Verify device is visible before creating datastore
	klog.Infof("Step 6: Verifying device %s is visible...", cinderVol.NAA)
	devicePath := fmt.Sprintf("/vmfs/devices/disks/%s", cinderVol.NAA)

	// Use a simpler check - just list disks and grep for our NAA
	checkCmd := fmt.Sprintf("ls /vmfs/devices/disks/ | grep %s", cinderVol.NAA)
	checkOutput, checkErr := c.ExecuteCommand(checkCmd)
	if checkErr != nil || checkOutput == "" || !strings.Contains(checkOutput, cinderVol.NAA) {
		klog.Warningf("Device not visible yet: %s (error: %v, output: '%s')", cinderVol.NAA, checkErr, checkOutput)

		// Show available devices for debugging
		allDisks, _ := c.ExecuteCommand("ls /vmfs/devices/disks/ | head -20")
		klog.Infof("Available devices (first 20): %s", allDisks)

		return nil, fmt.Errorf("device %s not visible on ESXi host after rescan - volume is connected to host group 'esxi-xcopy-hostgroup' on Pure, but ESXi can't see it yet. Try manual rescan or check Pure host group membership", cinderVol.NAA)
	}
	klog.Infof("✓ Device visible: %s", devicePath)

	// Step 7: Create VMFS datastore on the new volume
	klog.Infof("Step 7: Creating VMFS datastore %s on NAA %s...", datastoreName, cinderVol.NAA)
	err = c.CreateDatastore(datastoreName, cinderVol.NAA)
	if err != nil {
		return nil, fmt.Errorf("failed to create datastore: %w", err)
	}
	klog.Infof("✓ Created datastore: %s", datastoreName)

	// Step 7: Get datastore path
	datastorePath, err := c.GetDatastorePath(datastoreName)
	if err != nil {
		return nil, fmt.Errorf("failed to get datastore path: %w", err)
	}
	klog.Infof("Datastore path: %s", datastorePath)

	// Step 8: Clone VMDK to new datastore using VAAI XCOPY
	targetVMDK := fmt.Sprintf("%s/disk.vmdk", datastorePath)
	result.TargetVMDK = targetVMDK

	klog.Infof("Step 7: Starting VAAI XCOPY clone to %s...", targetVMDK)
	cloneStart := time.Now()
	task, err := c.StartVmkfstoolsClone(sourceVMDK, targetVMDK)
	if err != nil {
		return nil, fmt.Errorf("failed to start VAAI clone: %w", err)
	}

	// Step 9: Monitor clone progress
	tracker := NewCloneTracker(c, task, sourceVMDK, targetVMDK)
	tracker.SetPollInterval(2 * time.Second)

	err = tracker.WaitForCompletion()
	if err != nil {
		return nil, fmt.Errorf("VAAI clone failed: %w", err)
	}

	result.CloneDuration = time.Since(cloneStart)
	result.TotalDuration = time.Since(overallStart)
	result.ConversionStatus = "success"

	klog.Infof("✓ Conversion complete in %s (clone: %s)", result.TotalDuration, result.CloneDuration)
	klog.Infof("✓ VMDK cloned to Cinder volume %s (NAA: %s)", cinderVolName, result.TargetNAA)
	klog.Infof("✓ Volume ready for OpenStack - already in Cinder format")

	return result, nil
}
