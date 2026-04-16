package rush

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// StoreInVault finalizes a completely built and verified node_modules set into 
// the persistent local L1 cache on the SSD. It performs atomic stage-then-swap 
// renaming to prevent corrupted states if the process is killed midway.
func StoreInVault(hash string) error {
	tempVaultBase := filepath.Join(".rush-cache", hash+".tmp")
	finalVaultBase := filepath.Join(".rush-cache", hash)

	target := filepath.Join(tempVaultBase, "node_modules")
	os.MkdirAll(tempVaultBase, os.ModePerm)

	source, _ := filepath.Abs("node_modules")
	targetAbs, _ := filepath.Abs(target)

	// 1. Ensure target/staging area is clean
	os.RemoveAll(tempVaultBase)
	os.MkdirAll(tempVaultBase, os.ModePerm)

	// 2. Retry loop for "Access is denied" during move to staging
	var err error
	for i := 0; i < 5; i++ {
		err = os.Rename(source, targetAbs)
		if err == nil {
			break
		}
		fmt.Printf("Retry %d/5: node_modules move failed (probably locked), waiting...\n", i+1)
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		os.RemoveAll(tempVaultBase)
		return fmt.Errorf("failed to move node_modules to staging: %v", err)
	}

	// 3. Atomic Swap: Stage-then-Swap to final hash path
	os.RemoveAll(finalVaultBase)
	if err := os.Rename(tempVaultBase, finalVaultBase); err != nil {
		return fmt.Errorf("failed to finalize vault directory: %v", err)
	}

	// 4. Create Junction to the finalized location
	finalTargetAbs, _ := filepath.Abs(filepath.Join(finalVaultBase, "node_modules"))
	return makeJunction(finalTargetAbs, source)
}

// CopyfromVault pulls an existing node_modules from the L1 cache and projects it 
// into the current working directory utilizing a native file system junction.
// This allows sub-100ms instant restoration.
func CopyfromVault(hash string) {
	vault, _ := filepath.Abs(filepath.Join(".rush-cache", hash, "node_modules"))
	local, _ := filepath.Abs("node_modules")
	os.RemoveAll(local)

	makeJunction(vault, local)
}

// makeJunction provides a Windows-compatible method of creating directory symlinks
// bypassing the strict Administrator-only boundaries of standard Windows symlinks.
func makeJunction(target, source string) error {
	// os.Symlink on Windows requires admin/dev mode. Junctions do not.
	cmd := exec.Command("cmd", "/c", "mklink", "/j", source, target)
	return cmd.Run()
}
