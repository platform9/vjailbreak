// Copyright © 2024 The vjailbreak authors

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	esxissh "github.com/platform9/vjailbreak/v2v-helper/esxi-ssh"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage/pure"
)

func main() {
	fmt.Println("=== Testing Complete VAAI Clone Flow ===\n")

	// Get environment variables
	esxiHost := os.Getenv("ESXI_HOST")
	esxiUser := os.Getenv("ESXI_USER")
	esxiKeyPath := os.Getenv("ESXI_SSH_KEY_PATH")
	pureHost := os.Getenv("PURE_HOST")
	pureUser := os.Getenv("PURE_USER")
	purePassword := os.Getenv("PURE_PASSWORD")

	if esxiHost == "" || esxiUser == "" || esxiKeyPath == "" {
		fmt.Println("Missing ESXi credentials:")
		fmt.Println("  export ESXI_HOST=10.110.25.77")
		fmt.Println("  export ESXI_USER=root")
		fmt.Println("  export ESXI_SSH_KEY_PATH=~/.ssh/id_rsa")
		os.Exit(1)
	}

	if pureHost == "" || pureUser == "" || purePassword == "" {
		fmt.Println("Missing Pure credentials:")
		fmt.Println("  export PURE_HOST=<pure-array-ip>")
		fmt.Println("  export PURE_USER=<username>")
		fmt.Println("  export PURE_PASSWORD=<password>")
		os.Exit(1)
	}

	// Step 1: Connect to ESXi
	fmt.Println("Step 1: Connecting to ESXi...")
	privateKey, err := os.ReadFile(os.ExpandEnv(esxiKeyPath))
	if err != nil {
		fmt.Printf("Failed to read SSH key: %v\n", err)
		os.Exit(1)
	}

	esxiClient := esxissh.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := esxiClient.Connect(ctx, esxiHost, esxiUser, privateKey); err != nil {
		fmt.Printf("Failed to connect to ESXi: %v\n", err)
		os.Exit(1)
	}
	defer esxiClient.Disconnect()
	fmt.Println("✓ Connected to ESXi")

	// Step 2: Connect to Pure FlashArray
	fmt.Println("\nStep 2: Connecting to Pure FlashArray...")
	pureProvider := &pure.PureStorageProvider{}
	accessInfo := storage.StorageAccessInfo{
		Hostname:            pureHost,
		Username:            pureUser,
		Password:            purePassword,
		SkipSSLVerification: true,
	}
	if err := pureProvider.Connect(context.Background(), accessInfo); err != nil {
		fmt.Printf("Failed to connect to Pure: %v\n", err)
		os.Exit(1)
	}
	defer pureProvider.Disconnect()
	fmt.Println("✓ Connected to Pure FlashArray")

	// Step 3: Test VMDK → NAA resolution
	fmt.Println("\nStep 3: Testing VMDK → NAA resolution...")
	testVMDK := "/vmfs/volumes/pure-ds/pure-clone1/pure-clone1.vmdk"
	naaID, err := esxiClient.GetVMDKBackingNAA(testVMDK)
	if err != nil {
		fmt.Printf("Failed to get NAA from VMDK: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ VMDK %s\n", testVMDK)
	fmt.Printf("  → Backed by NAA: %s\n", naaID)

	// Step 4: Test NAA → Pure Volume resolution
	fmt.Println("\nStep 4: Testing NAA → Pure Volume resolution...")
	pureVolume, err := pureProvider.GetVolumeFromNAA(naaID)
	if err != nil {
		fmt.Printf("Failed to get Pure volume from NAA: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ NAA %s\n", naaID)
	fmt.Printf("  → Pure Volume: %s\n", pureVolume.Name)
	fmt.Printf("  → Serial: %s\n", pureVolume.SerialNumber)
	fmt.Printf("  → Size: %d bytes\n", pureVolume.Size)

	// Step 5: Test VAAI clone + Cinder conversion (VMDK → Cinder Volume)
	fmt.Println("\nStep 5: Testing VAAI clone + Cinder conversion (dynamic datastore)...")
	sourceVMDK := "/vmfs/volumes/pure-ds/pure-clone1/pure-clone1.vmdk"

	// Generate a Cinder volume ID (in production, this comes from OpenStack)
	cinderVolumeID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"

	fmt.Printf("  Source VMDK: %s\n", sourceVMDK)
	fmt.Printf("  Cinder Volume ID: %s\n", cinderVolumeID)
	fmt.Printf("  ESXi Host: %s\n\n", esxiHost)
	fmt.Println("  This will:")
	fmt.Println("  1. Create Pure volume: volume-<uuid>-cinder")
	fmt.Println("  2. Create VMFS datastore on new volume")
	fmt.Println("  3. VAAI XCOPY clone to new datastore")
	fmt.Println()

	// Perform complete conversion with dynamic datastore creation
	result, err := esxiClient.ConvertVMDKToCinder(
		context.Background(),
		sourceVMDK,
		cinderVolumeID,
		esxiHost,
		pureProvider,
	)
	if err != nil {
		fmt.Printf("✗ Conversion failed: %v\n", err)
		os.Exit(1)
	}

	// Display results
	fmt.Println("\n=== Conversion Results ===")
	fmt.Printf("✓ VAAI clone completed in: %s\n", result.CloneDuration.Round(time.Second))
	fmt.Printf("✓ Total conversion time: %s\n", result.TotalDuration.Round(time.Second))
	fmt.Printf("\nSource:\n")
	fmt.Printf("  VMDK: %s\n", result.SourceVMDK)
	fmt.Printf("\nTarget:\n")
	fmt.Printf("  VMDK: %s\n", result.TargetVMDK)
	fmt.Printf("  NAA:  %s\n", result.TargetNAA)
	fmt.Printf("\nCinder Volume:\n")
	fmt.Printf("  Name: %s\n", result.CinderVolName)
	fmt.Printf("  ID:   %s\n", result.CinderVolumeID)

	// Step 6: Verify we can resolve the Cinder volume
	fmt.Println("\nStep 6: Verifying Cinder volume resolution...")
	cinderVol, err := pureProvider.ResolveCinderVolumeToLUN(cinderVolumeID)
	if err != nil {
		fmt.Printf("✗ Failed to resolve Cinder volume: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Resolved Cinder volume: %s\n", cinderVol.Name)
	fmt.Printf("  NAA: %s\n", cinderVol.NAA)
	fmt.Printf("  Serial: %s\n", cinderVol.SerialNumber)

	if cinderVol.NAA != result.TargetNAA {
		fmt.Printf("✗ ERROR: NAA mismatch! Expected %s, got %s\n", result.TargetNAA, cinderVol.NAA)
		os.Exit(1)
	}

	fmt.Println("\n=== All Tests Passed! ===")
	fmt.Printf("\n✓ Complete VMDK → Cinder conversion in %s\n", result.TotalDuration.Round(time.Second))
	fmt.Println("\nWhat happened:")
	fmt.Println("  1. Created Pure volume with Cinder name")
	fmt.Println("  2. Rescanned ESXi storage to detect new volume")
	fmt.Println("  3. Created VMFS datastore on new Pure volume")
	fmt.Println("  4. VAAI XCOPY cloned VMDK to new datastore (hardware accelerated)")
	fmt.Println("  5. Verified Cinder volume resolution works")
	fmt.Println("\nReady for OpenStack:")
	fmt.Printf("  - Volume %s is ready to be mapped to OpenStack compute host\n", result.CinderVolName)
	fmt.Printf("  - NAA %s can be attached as Cinder volume\n", result.TargetNAA)
	fmt.Println("  - Original VM on ESXi is unchanged and still running")
	fmt.Println("\nCleanup (when done testing):")
	fmt.Printf("  - Delete datastore: ssh root@%s 'esxcli storage filesystem unmount -l cinder-%s'\n", esxiHost, cinderVolumeID[:8])
	fmt.Printf("  - Delete Pure volume: (use Pure GUI or API to delete %s)\n", result.CinderVolName)
}
