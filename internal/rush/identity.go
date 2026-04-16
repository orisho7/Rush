package rush

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
)

// getNodeVersion shells out to the 'node -v' command and returns the local Node.js version.
// This ensures that cache payloads are strictly tied to the runtime version.
func getNodeVersion() string {
	out, err := exec.Command("node", "-v").Output()
	if err != nil {
		return "unknown"
	}
	return string(out)
}

// generateIdentityHash orchestrates the creation of the deterministic project build hash.
// It incorporates the OS, Architecture, Node Version, and the raw contents of local lockfiles
// to prevent environmental cache poisoning or mismatched binaries.
func generateIdentityHash(nodeVersion string) (bool, string, string) {
	h := sha256.New()
	tool := "npm" // default

	// Encode deterministic environment factors clearly to prevent collisions
	fmt.Fprintf(h, "%s:%s:%s:", runtime.GOOS, runtime.GOARCH, nodeVersion)

	var foundAnyLock bool
	for _, filename_ := range filenames {
		f, err := os.Open(filename_)
		if err != nil {
			continue
		}
		foundAnyLock = true
		if filename_ == "yarn.lock" {
			tool = "yarn"
		} else if filename_ == "pnpm-lock.yaml" {
			tool = "pnpm"
		}

		_, _ = io.Copy(h, f)
		f.Close()
	}

	return foundAnyLock, tool, hex.EncodeToString(h.Sum(nil))
}
